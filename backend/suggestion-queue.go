package backend

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// closed once the Ollama health check succeeds; workers block until then
var ollamaReady = make(chan struct{})

func isOllamaReady() bool {
	select {
	case <-ollamaReady:
		return true
	default:
		return false
	}
}

func waitForOllama() {
	addr, _ := effectiveOllamaConfig()
	Log.Infow("ollama: waiting for health check", "url", addr)
	for {
		addr, _ = effectiveOllamaConfig()
		resp, err := http.Get(addr + "/api/tags")
		if err != nil {
			Log.Infow("ollama: not ready, retrying in 2s", "error", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				Log.Info("ollama: health check passed, suggestions enabled")
				close(ollamaReady)
				BroadcastAll("suggestion_server_online")
				return
			}
			Log.Infow("ollama: not ready, retrying in 2s", "status", resp.StatusCode)
		}
		time.Sleep(2 * time.Second)
	}
}

const maxSuggestionRetries = 5

type SuggestionJob struct {
	Model
	ArtifactID  uint   `gorm:"not null;index:idx_suggestion_job_dedup"`
	OllamaModel string `gorm:"not null;index:idx_suggestion_job_dedup"`
	OwnerID     *uint  `gorm:"index"`
	Username    string
	Status      string `gorm:"not null;index"`
	ErrorMsg    string
	RetryCount  int
	Source      string // "store", "backfill"
	DurationMs  *int64
}

var suggestionRetryTrigger = make(chan struct{}, 1)

func triggerSuggestionRetry() {
	select {
	case suggestionRetryTrigger <- struct{}{}:
	default:
	}
}

func retryFailedSuggestionJobs() {
	var jobs []SuggestionJob
	db.Where("status = ? AND retry_count < ?", JobStatusFailed, maxSuggestionRetries).Find(&jobs)
	for _, j := range jobs {
		db.Model(&j).Updates(map[string]interface{}{
			"status":      JobStatusPending,
			"error_msg":   "",
			"retry_count": j.RetryCount + 1,
		})
		Log.Infow("retrying failed suggestion job", "jobID", j.ID, "attempt", j.RetryCount+1)
		select {
		case suggestionJobQueue <- j.ID:
		default:
		}
	}
}

var suggestionJobQueue = make(chan uint, 4096)

// EnqueueSuggestionJob creates a job if no pending/processing job exists for the same artifact+model.
func EnqueueSuggestionJob(artifactID uint, ownerID *uint, username, ollamaModel, source string) {
	if db == nil {
		return
	}

	var count int64
	db.Model(&SuggestionJob{}).
		Where("artifact_id = ? AND ollama_model = ? AND status IN ?",
			artifactID, ollamaModel, []string{JobStatusPending, JobStatusProcessing}).
		Count(&count)
	if count > 0 {
		return
	}

	job := SuggestionJob{
		ArtifactID:  artifactID,
		OllamaModel: ollamaModel,
		OwnerID:     ownerID,
		Username:    username,
		Status:      JobStatusPending,
		Source:      source,
	}
	if err := db.Create(&job).Error; err != nil {
		Log.Errorw("failed to enqueue suggestion job", "error", err)
		return
	}
	if !isOllamaReady() {
		BroadcastToUser(username, "suggestion_server_offline")
	}

	select {
	case suggestionJobQueue <- job.ID:
	default:
		Log.Warnw("suggestion job queue full; job saved to DB for recovery", "jobID", job.ID)
	}
}

// StartSuggestionWorkers recovers pending DB jobs and starts worker goroutines.
func StartSuggestionWorkers() {
	go waitForOllama()

	go func() {
		db.Model(&SuggestionJob{}).Where("status = ?", JobStatusProcessing).Update("status", JobStatusPending)

		var jobs []SuggestionJob
		db.Where("status = ?", JobStatusPending).Find(&jobs)
		for _, j := range jobs {
			select {
			case suggestionJobQueue <- j.ID:
			default:
			}
		}

		for range time.Tick(30 * time.Second) {
			var pending []SuggestionJob
			db.Where("status = ?", JobStatusPending).Find(&pending)
			for _, j := range pending {
				select {
				case suggestionJobQueue <- j.ID:
				default:
				}
			}
		}
	}()

	go func() {
		<-ollamaReady
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-suggestionRetryTrigger:
			case <-ticker.C:
			}
			retryFailedSuggestionJobs()
		}
	}()

	n := cap(suggestionSemaphore)
	for i := 0; i < n; i++ {
		go suggestionWorkerLoop()
	}
	Log.Infof("suggestion queue: started %d workers", n)
}

func broadcastSuggestionProgress(job SuggestionJob) {
	msg := "suggestion_progress:" + strconv.FormatUint(uint64(job.ID), 10) + ":artifact:" + strconv.FormatUint(uint64(job.ArtifactID), 10)
	BroadcastToUser(job.Username, msg)
}

func suggestionWorkerLoop() {
	<-ollamaReady
	for jobID := range suggestionJobQueue {
		processSuggestionJob(jobID)
	}
}

func processSuggestionJob(jobID uint) {
	var job SuggestionJob
	if err := db.First(&job, jobID).Error; err != nil {
		return
	}

	result := db.Model(&SuggestionJob{}).
		Where("id = ? AND status = ?", job.ID, JobStatusPending).
		Update("status", JobStatusProcessing)
	if result.RowsAffected == 0 {
		return
	}

	// Fast dedup: skip if suggestion already exists for this artifact+model
	var count int64
	db.Model(&ArtifactSuggestion{}).
		Where("artifact_id = ? AND ollama_model = ?", job.ArtifactID, job.OllamaModel).
		Count(&count)
	if count > 0 {
		db.Model(&job).Update("status", JobStatusDone)
		broadcastSuggestionProgress(job)
		return
	}

	ctx := context.WithValue(dbCtx, usernameContextKey, job.Username)
	start := time.Now()
	genErr := processArtifactSuggestionJob(job.ArtifactID, job.OllamaModel, ctx)
	ms := time.Since(start).Milliseconds()

	if genErr != nil {
		db.Model(&job).Updates(map[string]interface{}{
			"status":      JobStatusFailed,
			"error_msg":   genErr.Error(),
			"duration_ms": ms,
		})
		Log.Errorw("suggestion job failed", "jobID", job.ID, "artifactID", job.ArtifactID, "error", genErr)
	} else {
		db.Model(&job).Updates(map[string]interface{}{
			"status":      JobStatusDone,
			"duration_ms": ms,
		})
		triggerSuggestionRetry()
	}

	broadcastSuggestionProgress(job)
}

func processArtifactSuggestionJob(artifactID uint, ollamaModel string, ctx context.Context) error {
	artifact, err := GetArtifactFromDB(artifactID)
	if err != nil {
		return err
	}
	iface, err := artifact.GetInterface()
	if err != nil {
		return nil // unsupported type, skip silently
	}
	img, ok := iface.(*Image)
	if !ok {
		return nil
	}

	data, err := img.GetOriginalContents()
	if err != nil {
		return err
	}

	addr, _ := effectiveOllamaConfig()
	suggestions, err := generateItemSuggestions(addr, ollamaModel, *data)
	if err != nil {
		return err
	}

	if err = saveSuggestion(artifactID, ollamaModel, suggestions); err != nil {
		return err
	}

	return saveSuggestionTextEmbedding(artifactID, suggestions, ctx)
}

func saveSuggestionTextEmbedding(artifactID uint, s ItemSuggestions, ctx context.Context) error {
	text := s.Name
	if s.Description != "" {
		if text != "" {
			text += " - "
		}
		text += s.Description
	}
	if text == "" {
		return nil
	}

	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return err
	}
	textModel, _, _, _ := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)

	vec, fullInput, err := GenerateTextDocumentEmbeddingsCtx(ctx, text)
	if err != nil {
		return err
	}
	_ = maxDims // dimensions are encoded in the stored vec length; saveEmbedding handles it

	return saveEmbedding(nil, &artifactID, vec, textModel, fullInput)
}

func backfillArtifactSuggestions() error {
	addr, model := effectiveOllamaConfig()
	if addr == "" || model == "" {
		Log.Warn("ollama not configured, skipping suggestion backfill")
		return nil
	}

	type artifactRow struct {
		ID      uint
		OwnerID *uint
	}
	var artifacts []artifactRow
	err := db.Model(&Artifact{}).
		Select("artifacts.id, artifacts.owner_id").
		Where("id NOT IN (SELECT artifact_id FROM artifact_suggestions WHERE ollama_model = ?)", model).
		Scan(&artifacts).Error
	if err != nil {
		Log.Errorw("backfill: failed to fetch artifacts for suggestions", "error", err)
		return err
	}

	// Load usernames for owner IDs
	ownerIDSet := map[uint]bool{}
	for _, a := range artifacts {
		if a.OwnerID != nil {
			ownerIDSet[*a.OwnerID] = true
		}
	}
	ownerIDs := make([]uint, 0, len(ownerIDSet))
	for id := range ownerIDSet {
		ownerIDs = append(ownerIDs, id)
	}
	userByID := map[uint]User{}
	if len(ownerIDs) > 0 {
		var users []User
		if err = db.Where("id IN ?", ownerIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				userByID[u.ID] = u
			}
		}
	}

	for _, a := range artifacts {
		var ownerID *uint
		username := ""
		if a.OwnerID != nil {
			ownerID = a.OwnerID
			if u, ok := userByID[*a.OwnerID]; ok {
				username = u.Username
			}
		}
		EnqueueSuggestionJob(a.ID, ownerID, username, model, "backfill")
	}

	Log.Infow("backfill: enqueued suggestion jobs", "count", len(artifacts))
	return nil
}

// SuggestionBackfillCount returns the number of artifacts without a cached suggestion for the given model.
func SuggestionBackfillCount(model string) (int64, error) {
	var count int64
	err := db.Model(&Artifact{}).
		Where("id NOT IN (SELECT artifact_id FROM artifact_suggestions WHERE ollama_model = ?)", model).
		Count(&count).Error
	return count, err
}
