package backend

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// --- GET /api/artifact/{id}/suggestion ---

var GetArtifactSuggestionOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/artifact/{id}/suggestion",
	DefaultStatus: http.StatusOK,
	OperationID:   "get-artifact-suggestion",
}

type ArtifactSuggestionBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Quantity    *uint  `json:"quantity,omitempty"`
	OllamaModel string `json:"ollamaModel"`
}

func GetArtifactSuggestionHandler(_ context.Context, input *struct {
	ID uint `path:"id"`
}) (output *struct{ Body ArtifactSuggestionBody }, err error) {
	_, model, _ := effectiveOllamaConfig()
	s, err := GetArtifactSuggestion(input.ID, model)
	if err != nil {
		return
	}
	if s == nil {
		err = huma.Error404NotFound("no suggestion cached for this artifact")
		return
	}
	output = &struct{ Body ArtifactSuggestionBody }{Body: ArtifactSuggestionBody{
		Name:        s.Name,
		Description: s.Description,
		Quantity:    s.Quantity,
		OllamaModel: s.OllamaModel,
	}}
	return
}

// --- Suggestion jobs ---

type SuggestionJobInfo struct {
	ID          uint      `json:"id"`
	ArtifactID  uint      `json:"artifactID"`
	RecordID    *uint     `json:"recordID,omitempty"`
	OllamaModel string    `json:"ollamaModel"`
	Username    string    `json:"username"`
	Status      string    `json:"status"`
	ErrorMsg    string    `json:"errorMsg,omitempty"`
	RetryCount  int       `json:"retryCount"`
	Source      string    `json:"source"`
	DurationMs  *int64    `json:"durationMs,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SuggestionJobsPage struct {
	Jobs  []SuggestionJobInfo `json:"jobs"`
	Total int64               `json:"total"`
}

var ListSuggestionJobsOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/suggestions/jobs",
	DefaultStatus: http.StatusOK,
	OperationID:   "list-suggestion-jobs",
}

func ListSuggestionJobs(ctx context.Context, input *struct {
	All    bool   `query:"all" required:"false"`
	Status string `query:"status" required:"false"`
	Limit  int    `query:"limit" required:"false" default:"50"`
	Offset int    `query:"offset" required:"false" default:"0"`
}) (output *struct{ Body SuggestionJobsPage }, err error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	showAll := input.All && uc.IsAdmin

	q := db.Model(&SuggestionJob{})
	if !showAll && username != "" && uc.ID > 0 {
		q = q.Where("owner_id = ?", uc.ID)
	}
	if input.Status != "" {
		q = q.Where("status = ?", input.Status)
	}

	var total int64
	if err = q.Count(&total).Error; err != nil {
		return
	}

	var jobs []SuggestionJob
	order := "CASE status WHEN 'processing' THEN 0 WHEN 'pending' THEN 1 WHEN 'failed' THEN 2 ELSE 3 END, created_at DESC"
	if err = q.Order(order).Limit(input.Limit).Offset(input.Offset).Find(&jobs).Error; err != nil {
		return
	}

	// Resolve artifact → record mappings in one query.
	artifactIDs := make([]uint, len(jobs))
	for i, j := range jobs {
		artifactIDs[i] = j.ArtifactID
	}
	type artRecord struct {
		ID       uint
		RecordID *uint
	}
	var arts []artRecord
	db.Model(&Artifact{}).Select("id, record_id").Where("id IN ?", artifactIDs).Scan(&arts)
	recordByArtifact := make(map[uint]*uint, len(arts))
	for _, a := range arts {
		recordByArtifact[a.ID] = a.RecordID
	}

	infos := make([]SuggestionJobInfo, len(jobs))
	for i, j := range jobs {
		infos[i] = SuggestionJobInfo{
			ID:          j.ID,
			ArtifactID:  j.ArtifactID,
			RecordID:    recordByArtifact[j.ArtifactID],
			OllamaModel: j.OllamaModel,
			Username:    j.Username,
			Status:      j.Status,
			ErrorMsg:    j.ErrorMsg,
			RetryCount:  j.RetryCount,
			Source:      j.Source,
			DurationMs:  j.DurationMs,
			CreatedAt:   j.CreatedAt,
			UpdatedAt:   j.UpdatedAt,
		}
	}
	output = &struct{ Body SuggestionJobsPage }{Body: SuggestionJobsPage{Jobs: infos, Total: total}}
	return
}

var ResetStuckSuggestionJobsOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/suggestions/jobs/reset",
	DefaultStatus: http.StatusNoContent,
	OperationID:   "reset-stuck-suggestion-jobs",
}

func ResetStuckSuggestionJobs(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	var jobs []SuggestionJob
	if err := db.Where("status = ?", JobStatusProcessing).Find(&jobs).Error; err != nil {
		return nil, err
	}
	for _, j := range jobs {
		if _, active := activeSuggestionJobs.Load(j.ID); !active {
			db.Model(&j).Update("status", JobStatusPending)
		}
	}
	return nil, nil
}

var DeletePendingSuggestionJobsOperation = huma.Operation{
	Method:        http.MethodDelete,
	Path:          "/api/suggestions/jobs",
	DefaultStatus: http.StatusNoContent,
	OperationID:   "delete-pending-suggestion-jobs",
}

func DeletePendingSuggestionJobs(ctx context.Context, input *struct {
	All    bool   `query:"all" required:"false"`
	Status string `query:"status" required:"false" default:"pending"`
}) (*struct{}, error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	status := input.Status
	if status == "" {
		status = JobStatusPending
	}

	q := db.Where("status = ?", status)
	if !(input.All && uc.IsAdmin) {
		q = q.Where("owner_id = ?", uc.ID)
	}
	return nil, q.Delete(&SuggestionJob{}).Error
}

var DeleteSuggestionJobOperation = huma.Operation{
	Method:        http.MethodDelete,
	Path:          "/api/suggestions/jobs/{id}",
	DefaultStatus: http.StatusNoContent,
	OperationID:   "delete-suggestion-job",
}

func DeleteSuggestionJob(ctx context.Context, input *struct {
	ID uint `path:"id"`
}) (*struct{}, error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	q := db.Where("id = ? AND status = ?", input.ID, JobStatusPending)
	if !uc.IsAdmin {
		q = q.Where("owner_id = ?", uc.ID)
	}

	result := q.Delete(&SuggestionJob{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, huma.Error404NotFound("job not found or not deletable")
	}
	return nil, nil
}
