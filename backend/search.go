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
	es, partial, err := GetArtifactEmbeddings(ctx, artifactRecordMap)
	if err != nil {
		return
	}

	searchEmbeddings, err := GenerateImageQueryEmbeddingsCtx(ctx, search)
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

func SearchByRecord(ctx context.Context, search string, scopedIDs []uint) (recordResults []struct {
	id    uint
	score float64
}, partial bool, err error) {
	es, partial, err := GetRecordEmbeddings(ctx, scopedIDs)
	if err != nil {
		return
	}

	searchEmbeddings, err := GenerateTextQueryEmbeddingsCtx(ctx, search)
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
