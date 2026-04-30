package backend

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// closed once the Infinity health check succeeds; workers block until then
var infinityReady = make(chan struct{})

func isInfinityReady() bool {
	select {
	case <-infinityReady:
		return true
	default:
		return false
	}
}

func waitForInfinity() {
	Log.Infow("infinity: waiting for health check", "url", infinityAddress+"/health")
	for {
		resp, err := http.Get(infinityAddress + "/health")
		if err != nil {
			Log.Infow("infinity: not ready, retrying in 2s", "error", err)
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				Log.Info("infinity: health check passed, embeddings enabled")
				close(infinityReady)
				BroadcastAll("embedding_server_online")
				return
			}
			Log.Infow("infinity: not ready, retrying in 2s", "status", resp.StatusCode)
		}
		time.Sleep(2 * time.Second)
	}
}

const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusDone       = "done"
	JobStatusFailed     = "failed"

	JobTypeArtifact = "artifact"
	JobTypeRecord   = "record"

	maxEmbeddingRetries = 5
)

type EmbeddingJob struct {
	Model
	JobType    string `gorm:"not null;index:idx_embedding_job_dedup"`
	TargetID   uint   `gorm:"not null;index:idx_embedding_job_dedup"`
	OwnerID    *uint  `gorm:"index"`
	Username   string
	Status     string `gorm:"not null;index"`
	ErrorMsg   string
	RetryCount int
	EmbedModel string `gorm:"not null;index:idx_embedding_job_dedup"`
	Source     string // "store", "search", "backfill"
}

// retryTrigger is signalled after each successful job or every 10 s, whichever comes first.
// Buffered at 1 so rapid successes coalesce into a single retry scan.
var retryTrigger = make(chan struct{}, 1)

func triggerRetry() {
	select {
	case retryTrigger <- struct{}{}:
	default:
	}
}

func retryFailedJobs() {
	var jobs []EmbeddingJob
	db.Where("status = ? AND retry_count < ?", JobStatusFailed, maxEmbeddingRetries).Find(&jobs)
	for _, j := range jobs {
		db.Model(&j).Updates(map[string]interface{}{
			"status":      JobStatusPending,
			"error_msg":   "",
			"retry_count": j.RetryCount + 1,
		})
		Log.Infow("retrying failed embedding job", "jobID", j.ID, "attempt", j.RetryCount+1)
		select {
		case embeddingJobQueue <- j.ID:
		default:
		}
	}
}

var embeddingJobQueue = make(chan uint, 4096)
var embeddingSearchJobQueue = make(chan uint, 4096)

// EnqueueEmbeddingJob creates a job if no pending/processing job exists for the same target+model.
// the worker fast-path handles the case where the embedding already exists.
func EnqueueEmbeddingJob(jobType string, targetID uint, ownerID *uint, username, embedModel, source string) {
	if db == nil {
		return
	}

	var count int64
	db.Model(&EmbeddingJob{}).
		Where("job_type = ? AND target_id = ? AND embed_model = ? AND status IN ?",
			jobType, targetID, embedModel, []string{JobStatusPending, JobStatusProcessing}).
		Count(&count)
	if count > 0 {
		return
	}

	job := EmbeddingJob{
		JobType:    jobType,
		TargetID:   targetID,
		OwnerID:    ownerID,
		Username:   username,
		Status:     JobStatusPending,
		EmbedModel: embedModel,
		Source:     source,
	}
	if err := db.Create(&job).Error; err != nil {
		Log.Errorw("failed to enqueue embedding job", "error", err)
		return
	}
	if !isInfinityReady() {
		BroadcastToUser(username, "embedding_server_offline")
	}

	select {
	case embeddingJobQueue <- job.ID:
	default:
		Log.Warnw("embedding job queue full; job saved to DB for recovery", "jobID", job.ID)
	}
}

// StartEmbeddingWorkers recovers pending DB jobs and starts worker goroutines.
func StartEmbeddingWorkers() {
	go waitForInfinity()

	go func() {
		// Reset interrupted processing jobs back to pending
		db.Model(&EmbeddingJob{}).Where("status = ?", JobStatusProcessing).Update("status", JobStatusPending)

		var jobs []EmbeddingJob
		db.Where("status = ?", JobStatusPending).Find(&jobs)
		for _, j := range jobs {
			select {
			case embeddingJobQueue <- j.ID:
			default:
			}
		}

		// Periodic scan rescues jobs that couldn't fit in channel on enqueue
		for range time.Tick(30 * time.Second) {
			var pending []EmbeddingJob
			db.Where("status = ?", JobStatusPending).Find(&pending)
			for _, j := range pending {
				select {
				case embeddingJobQueue <- j.ID:
				default:
				}
			}
		}
	}()

	go func() {
		<-infinityReady
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-retryTrigger:
			case <-ticker.C:
			}
			retryFailedJobs()
		}
	}()

	n := cap(embeddingSemaphore)
	for i := 0; i < n; i++ {
		go embeddingWorkerLoop()
	}
	Log.Infof("embedding queue: started %d workers", n)
}

func broadcastEmbeddingProgress(job EmbeddingJob) {
	msg := "embedding_progress:" + job.JobType + ":" + strconv.FormatUint(uint64(job.TargetID), 10)
	Log.Infow("broadcasting embedding progress", "jobID", job.ID, "username", job.Username, "msg", msg)
	BroadcastToUser(job.Username, msg)
}

func embeddingWorkerLoop() {
	<-infinityReady
	for jobID := range embeddingJobQueue {
		processEmbeddingJob(jobID)
	}
}

func processEmbeddingJob(jobID uint) {
	var job EmbeddingJob
	if err := db.First(&job, jobID).Error; err != nil {
		return
	}

	// Atomically claim the job; skip if another path already claimed it
	result := db.Model(&EmbeddingJob{}).
		Where("id = ? AND status = ?", job.ID, JobStatusPending).
		Update("status", JobStatusProcessing)
	if result.RowsAffected == 0 {
		return
	}

	// Fast dedup: skip if embedding already exists for this target+model
	var count int64
	if job.JobType == JobTypeArtifact {
		db.Model(&Embedding{}).Where("artifact_id = ? AND embed_model = ?", job.TargetID, job.EmbedModel).Count(&count)
	} else {
		db.Model(&Embedding{}).Where("record_id = ? AND embed_model = ?", job.TargetID, job.EmbedModel).Count(&count)
	}
	if count > 0 {
		db.Model(&job).Update("status", JobStatusDone)
		broadcastEmbeddingProgress(job)
		return
	}

	ctx := context.WithValue(dbCtx, usernameContextKey, job.Username)
	var genErr error
	if job.JobType == JobTypeArtifact {
		genErr = processArtifactEmbeddingJob(job.TargetID, ctx)
	} else {
		genErr = processRecordEmbeddingJob(job.TargetID, ctx)
	}

	if genErr != nil {
		db.Model(&job).Updates(map[string]interface{}{
			"status":    JobStatusFailed,
			"error_msg": genErr.Error(),
		})
		Log.Errorw("embedding job failed", "jobID", job.ID, "type", job.JobType, "targetID", job.TargetID, "error", genErr)
	} else {
		db.Model(&job).Update("status", JobStatusDone)
		triggerRetry()
	}

	broadcastEmbeddingProgress(job)
}

func processArtifactEmbeddingJob(artifactID uint, ctx context.Context) error {
	a, err := GetArtifactFromDB(artifactID)
	if err != nil {
		return err
	}
	iface, err := a.GetInterface()
	if err != nil {
		return nil // unsupported type, skip silently
	}
	img, ok := iface.(*Image)
	if !ok {
		return nil
	}
	return img.GenerateEmbeddings(ctx)
}

func processRecordEmbeddingJob(recordID uint, ctx context.Context) error {
	var record Record
	if err := db.Select("id, title, reference_number, description").First(&record, recordID).Error; err != nil {
		return err
	}
	_, err := record.GenerateEmbeddings(ctx)
	return err
}
