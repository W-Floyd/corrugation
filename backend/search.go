package backend

import (
	"context"
	"errors"

	"github.com/viterin/vek"
)

const (
	minimumImageToImageSearchConfidence float64 = 0.6
	minimumTextToImageSearchConfidence  float64 = 0.2
	minimumTextSearchConfidence         float64 = 0.9
	minimumSuggestionSearchConfidence   float64 = 0.2
)

// cosineSimilarity computes the cosine similarity between two vectors.
// This is more robust than raw dot product as it's independent of vector magnitude.
func compareEmbeddings(v1 []float64, v2 []float64) (result float64, err error) {
	if len(v1) != len(v2) {
		return 0, errors.New("vectors should have same length")
	}
	return vek.CosineSimilarity(v1, v2), nil
}

func SearchByArtifact(ctx context.Context, search string, artifactRecordMap map[uint]*uint) (recordResults []struct {
	id    uint
	score float64
}, partial bool, err error) {
	searchEmbeddings, err := GenerateImageQueryEmbeddingsCtx(ctx, search)
	if err != nil {
		return
	}

	es, partial, err := GetArtifactEmbeddings(ctx, artifactRecordMap, uint(len(searchEmbeddings)))
	if err != nil {
		return
	}

	for _, e := range es {
		if e.recordID == nil {
			continue
		}
		var p float64
		p, err = compareEmbeddings(e.embedding, searchEmbeddings)
		if err != nil {
			return
		}
		recordResults = append(recordResults, struct {
			id    uint
			score float64
		}{id: *e.recordID, score: p})
	}

	return
}

// SearchBySuggestion searches using text embeddings stored on artifacts
// (generated from Ollama suggestion content) rather than record fields.
func SearchBySuggestion(ctx context.Context, search string, artifactRecordMap map[uint]*uint) (recordResults []struct {
	id    uint
	score float64
}, partial bool, err error) {
	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, _, _ := effectiveInfinityConfig(user)
	maxDims := effectiveMaxEmbeddingDimensions(user)

	searchEmbeddings, err := GenerateTextQueryEmbeddingsCtx(ctx, search)
	if err != nil {
		return
	}

	artifactIDs := make([]uint, 0, len(artifactRecordMap))
	for id := range artifactRecordMap {
		artifactIDs = append(artifactIDs, id)
	}
	if len(artifactIDs) == 0 {
		return
	}

	q := db.Where("artifact_id IN ? AND embed_model = ?", artifactIDs, textModel).
		Where("artifact_id NOT IN (SELECT a.id FROM artifacts a JOIN records r ON r.id = a.record_id WHERE r.exclude_from_suggestion_search = 1)")
	dims := uint(len(searchEmbeddings))
	if dims > 0 {
		q = q.Where("dimensions = ?", dims)
	}
	if maxDims != nil {
		q = q.Where("dimensions <= ?", *maxDims)
	}
	var embeddings []Embedding
	if err = q.Find(&embeddings).Error; err != nil {
		return
	}

	// best score per record across all its artifacts
	bestScore := map[uint]float64{}
	for _, emb := range embeddings {
		if emb.ArtifactID == nil {
			continue
		}
		recordID := artifactRecordMap[*emb.ArtifactID]
		if recordID == nil {
			continue
		}

		var vec Embeddings
		if cached, ok := embeddingsCache.Load(emb.Hash); ok {
			vec = cached.(Embeddings)
		} else {
			vec, err = UnmarshalEmbeddings(emb.Data)
			if err != nil {
				err = nil
				continue
			}
			embeddingsCache.Store(emb.Hash, vec)
		}

		score, cmpErr := compareEmbeddings(vec, searchEmbeddings)
		if cmpErr != nil {
			continue
		}
		if score > bestScore[*recordID] {
			bestScore[*recordID] = score
		}
	}

	// enqueue suggestion jobs for artifacts that had no text embedding
	embeddedArtifacts := map[uint]bool{}
	for _, emb := range embeddings {
		if emb.ArtifactID != nil {
			embeddedArtifacts[*emb.ArtifactID] = true
		}
	}
	_, ollamaModel, _, _, _ := effectiveOllamaConfig()
	for artifactID, recordID := range artifactRecordMap {
		if recordID == nil || embeddedArtifacts[artifactID] {
			continue
		}
		EnqueueSuggestionJob(artifactID, nil, UsernameFromContext(ctx), ollamaModel, "search")
		partial = true
	}

	for recordID, score := range bestScore {
		recordResults = append(recordResults, struct {
			id    uint
			score float64
		}{id: recordID, score: score})
	}
	return
}

func SearchByRecord(ctx context.Context, search string, scopedIDs []uint) (recordResults []struct {
	id    uint
	score float64
}, partial bool, err error) {
	searchEmbeddings, err := GenerateTextQueryEmbeddingsCtx(ctx, search)
	if err != nil {
		return
	}

	es, partial, err := GetRecordEmbeddings(ctx, scopedIDs, uint(len(searchEmbeddings)))
	if err != nil {
		return
	}

	for id, e := range es {
		var p float64
		p, err = compareEmbeddings(e, searchEmbeddings)
		if err != nil {
			return
		}
		recordResults = append(recordResults, struct {
			id    uint
			score float64
		}{id: id, score: p})
	}

	return
}
