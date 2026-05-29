package backend

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

var GetEmbeddingProgressOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/embeddings/progress",
}

type EmbeddingProgress struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Done       int64 `json:"done"`
	Failed     int64 `json:"failed"`
	Total      int64 `json:"total"`
}

func GetEmbeddingProgress(ctx context.Context, _ *struct{}) (output *struct{ Body EmbeddingProgress }, err error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	q := db.Model(&EmbeddingJob{})
	if username != "" && uc.ID > 0 {
		q = q.Where("owner_id = ?", uc.ID)
	}

	type statusCount struct {
		Status string
		Count  int64
	}
	var counts []statusCount
	if err = q.Select("status, COUNT(*) as count").Group("status").Scan(&counts).Error; err != nil {
		return
	}

	var p EmbeddingProgress
	for _, c := range counts {
		switch c.Status {
		case JobStatusPending:
			p.Pending = c.Count
		case JobStatusProcessing:
			p.Processing = c.Count
		case JobStatusDone:
			p.Done = c.Count
		case JobStatusFailed:
			p.Failed = c.Count
		}
	}
	p.Total = p.Pending + p.Processing + p.Done + p.Failed

	output = &struct{ Body EmbeddingProgress }{Body: p}
	return
}

var GetSearchEmbeddingProgressOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/embeddings/search-progress",
}

type SearchEmbeddingProgress struct {
	Record struct {
		Complete []uint `json:"complete"`
		Pending  []uint `json:"pending"`
	} `json:"record"`
	Artifact struct {
		Complete []uint `json:"complete"`
		Pending  []uint `json:"pending"`
	} `json:"artifact"`
}

func GetSearchEmbeddingProgress(ctx context.Context, input *struct {
	ID                 int  `query:"id" required:"false" default:"-1"`
	Global             bool `query:"global" required:"false"`
	ChildrenDepth      int  `query:"childrenDepth" required:"false"`
	SearchImage        bool `query:"searchImage" required:"false"`
	SearchTextEmbedded bool `query:"searchTextEmbedded" required:"false"`
}) (output *struct{ Body SearchEmbeddingProgress }, err error) {
	// Resolve scope
	var id *uint
	if !input.Global {
		if input.ID >= 0 {
			v := uint(input.ID)
			id = &v
		} else {
			var zero uint = 0
			id = &zero
		}
	}
	var childrenDepth *int
	if input.ChildrenDepth != 0 {
		childrenDepth = &input.ChildrenDepth
	}

	records, _, err := GetRecords(ctx, id, childrenDepth, nil, nil, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{q: "Artifacts", h: func(db gorm.PreloadBuilder) error { db.Select("id", "record_id"); return nil }},
	}, nil)
	if err != nil {
		return
	}

	recordIDs := make([]uint, 0, len(records))
	artifactIDs := make([]uint, 0)
	for _, r := range records {
		recordIDs = append(recordIDs, r.ID)
		for _, a := range r.Artifacts {
			if a != nil {
				artifactIDs = append(artifactIDs, a.ID)
			}
		}
	}

	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, imageModel, _, _ := effectiveInfinityConfig(user)

	var p SearchEmbeddingProgress

	if input.SearchTextEmbedded && len(recordIDs) > 0 {
		var indexed []uint
		db.Model(&Embedding{}).
			Where("record_id IN ? AND embed_model = ?", recordIDs, textModel).
			Pluck("record_id", &indexed)
		var pending []uint
		db.Model(&EmbeddingJob{}).
			Where("job_type = ? AND target_id IN ? AND embed_model = ? AND status IN ?",
				JobTypeRecord, recordIDs, textModel, []string{JobStatusPending, JobStatusProcessing}).
			Where("username = ?", UsernameFromContext(ctx)).
			Distinct("target_id").Pluck("target_id", &pending)
		p.Record.Complete = append(p.Record.Complete, indexed...)
		p.Record.Pending = append(p.Record.Pending, pending...)
	}

	if input.SearchImage && len(artifactIDs) > 0 {
		var indexed []uint
		db.Model(&Embedding{}).
			Where("artifact_id IN ? AND embed_model = ?", artifactIDs, imageModel).
			Pluck("artifact_id", &indexed)
		var pending []uint
		db.Model(&EmbeddingJob{}).
			Where("job_type = ? AND target_id IN ? AND embed_model = ? AND status IN ?",
				JobTypeArtifact, artifactIDs, imageModel, []string{JobStatusPending, JobStatusProcessing}).
			Where("username = ?", UsernameFromContext(ctx)).
			Distinct("target_id").Pluck("target_id", &pending)
		p.Artifact.Complete = append(p.Artifact.Complete, indexed...)
		p.Artifact.Pending = append(p.Artifact.Pending, pending...)
	}

	output = &struct{ Body SearchEmbeddingProgress }{Body: p}
	return
}

var ListEmbeddingJobsOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/embeddings/jobs",
}

type EmbeddingJobInfo struct {
	ID         uint      `json:"id"`
	JobType    string    `json:"jobType"`
	TargetID   uint      `json:"targetID"`
	Username   string    `json:"username"`
	Status     string    `json:"status"`
	ErrorMsg   string    `json:"errorMsg,omitempty"`
	RetryCount int       `json:"retryCount"`
	EmbedModel string    `json:"embedModel"`
	Dimensions *uint     `json:"dimensions,omitempty"`
	Source     string    `json:"source"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type EmbeddingJobsPage struct {
	Jobs  []EmbeddingJobInfo `json:"jobs"`
	Total int64              `json:"total"`
}

func ListEmbeddingJobs(ctx context.Context, input *struct {
	All    bool   `query:"all" required:"false"`
	Status string `query:"status" required:"false"`
	Limit  int    `query:"limit" required:"false" default:"50"`
	Offset int    `query:"offset" required:"false" default:"0"`
}) (output *struct{ Body EmbeddingJobsPage }, err error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	showAll := input.All && uc.IsAdmin

	q := db.Model(&EmbeddingJob{})
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

	var jobs []EmbeddingJob
	order := "CASE status WHEN 'processing' THEN 0 WHEN 'pending' THEN 1 WHEN 'failed' THEN 2 ELSE 3 END, created_at DESC"
	if err = q.Order(order).Limit(input.Limit).Offset(input.Offset).Find(&jobs).Error; err != nil {
		return
	}

	infos := make([]EmbeddingJobInfo, len(jobs))
	for i, j := range jobs {
		infos[i] = EmbeddingJobInfo{
			ID:         j.ID,
			JobType:    j.JobType,
			TargetID:   j.TargetID,
			Username:   j.Username,
			Status:     j.Status,
			ErrorMsg:   j.ErrorMsg,
			RetryCount: j.RetryCount,
			EmbedModel: j.EmbedModel,
			Dimensions: j.Dimensions,
			Source:     j.Source,
			CreatedAt:  j.CreatedAt,
			UpdatedAt:  j.UpdatedAt,
		}
	}
	output = &struct{ Body EmbeddingJobsPage }{Body: EmbeddingJobsPage{Jobs: infos, Total: total}}
	return
}

var DeletePendingEmbeddingJobsOperation = huma.Operation{
	Method:        http.MethodDelete,
	Path:          "/api/embeddings/jobs",
	DefaultStatus: http.StatusNoContent,
}

func DeletePendingEmbeddingJobs(ctx context.Context, input *struct {
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
	return nil, q.Delete(&EmbeddingJob{}).Error
}

var DeleteEmbeddingJobOperation = huma.Operation{
	Method:        http.MethodDelete,
	Path:          "/api/embeddings/jobs/{id}",
	DefaultStatus: http.StatusNoContent,
}

func DeleteEmbeddingJob(ctx context.Context, input *struct {
	ID uint `path:"id"`
}) (*struct{}, error) {
	username := UsernameFromContext(ctx)
	uc, _ := loadUser(username)

	q := db.Where("id = ? AND status = ?", input.ID, JobStatusPending)
	if !uc.IsAdmin {
		q = q.Where("owner_id = ?", uc.ID)
	}

	result := q.Delete(&EmbeddingJob{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, huma.Error404NotFound("job not found or not deletable")
	}
	return nil, nil
}

var InvalidateUserEmbeddingsOperation = huma.Operation{
	Method:        http.MethodDelete,
	Path:          "/api/embeddings/user",
	DefaultStatus: http.StatusNoContent,
}

func InvalidateUserEmbeddings(ctx context.Context, _ *struct{}) (*struct{}, error) {
	_, _, userID, err := UserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Delete embeddings for records owned by this user
	err = db.Where("record_id IN (SELECT id FROM records WHERE owner_id = ? AND deleted_at IS NULL)", userID).
		Delete(&Embedding{}).Error
	if err != nil {
		return nil, err
	}

	// Delete embeddings for artifacts owned by this user
	err = db.Where("artifact_id IN (SELECT id FROM artifacts WHERE owner_id = ? AND deleted_at IS NULL)", userID).
		Delete(&Embedding{}).Error
	if err != nil {
		return nil, err
	}

	return nil, nil
}
