package backend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

var GetBackfillPreviewOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/backfill/preview",
}

type BackfillPreview struct {
	LegacyEmbeddings int64 `json:"legacyEmbeddings"`
	Records          int64 `json:"records"`
	Artifacts        int64 `json:"artifacts"`
	Suggestions      int64 `json:"suggestions"`
}

func GetBackfillPreview(ctx context.Context, _ *struct{}) (output *struct{ Body BackfillPreview }, err error) {
	if err = requireAdmin(ctx); err != nil {
		return
	}

	var p BackfillPreview
	if err = db.Model(&Embedding{}).Unscoped().
		Where("substr(data, 1, 1) = X'5B'").
		Count(&p.LegacyEmbeddings).Error; err != nil {
		return
	}
	if err = db.Model(&Record{}).
		Where("id NOT IN (SELECT DISTINCT record_id FROM embeddings WHERE record_id IS NOT NULL AND deleted_at IS NULL)").
		Where("(title IS NOT NULL AND title != '') OR (reference_number IS NOT NULL AND reference_number != '') OR (description IS NOT NULL AND description != '')").
		Count(&p.Records).Error; err != nil {
		return
	}
	if err = db.Model(&Artifact{}).
		Where("id NOT IN (SELECT DISTINCT artifact_id FROM embeddings WHERE artifact_id IS NOT NULL AND deleted_at IS NULL)").
		Count(&p.Artifacts).Error; err != nil {
		return
	}
	_, ollamaModel := effectiveOllamaConfig()
	p.Suggestions, err = SuggestionBackfillCount(ollamaModel)
	if err != nil {
		return
	}

	output = &struct{ Body BackfillPreview }{Body: p}
	return
}

var RunRecordBackfillOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/backfill/records",
	DefaultStatus: http.StatusNoContent,
}

func RunRecordBackfill(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	go backfillRecordEmbeddings()
	return nil, nil
}

var RunArtifactBackfillOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/backfill/artifacts",
	DefaultStatus: http.StatusNoContent,
}

func RunArtifactBackfill(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	go backfillArtifactEmbeddings()
	return nil, nil
}

var RunSuggestionsBackfillOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/backfill/suggestions",
	DefaultStatus: http.StatusNoContent,
	OperationID:   "run-suggestions-backfill",
}

func RunSuggestionsBackfill(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	go backfillArtifactSuggestions()
	return nil, nil
}

var RunLegacyEmbeddingsBackfillOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/backfill/legacy-embeddings",
	DefaultStatus: http.StatusNoContent,
}

func RunLegacyEmbeddingsBackfill(ctx context.Context, _ *struct{}) (*struct{}, error) {
	if err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	go BackfillLegacyEmbeddingsOnStart()
	return nil, nil
}
