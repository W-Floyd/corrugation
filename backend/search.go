package backend

import (
	"context"
	"errors"

	"gonum.org/v1/gonum/blas/blas64"
)

const (
	minimumImageToImageSearchConfidence float64 = 0.55
	minimumTextToImageSearchConfidence  float64 = 0.2
	minimumTextSearchConfidence         float64 = 0.9
)

func dotProduct(v1 []float64, v2 []float64) (result float64, err error) {
	if len(v1) != len(v2) {
		return 0, errors.New("vectors should have same length")
	}
	return blas64.Implementation().Ddot(len(v1), v1, 1, v2, 1), nil
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
		p, err = dotProduct(e.embedding, searchEmbeddings)
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
		p, err = dotProduct(e, searchEmbeddings)
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
