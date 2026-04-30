package backend

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

const maxSearchDepth = 100

type RecordInput struct {
	Quantity        *uint   `required:"false"`
	ReferenceNumber *string `required:"false"`
	Title           *string `required:"false"`
	Description     *string `required:"false"`
	ParentID        *uint   `required:"false"`
	Artifacts       []*uint `required:"false"`
}

type Record struct {
	Model

	Quantity        *uint   `json:",omitempty"`
	ReferenceNumber *string `json:",omitempty" gorm:"uniqueIndex:idx_owner_ref"`
	Title           *string `json:",omitempty" gorm:"index"`
	Description     *string `json:",omitempty"`

	Artifacts []*Artifact `json:",omitempty"`

	ParentID *uint   `json:",omitempty" gorm:"index"`
	Parent   *Record `gorm:"foreignKey:ParentID" json:"-"`

	OwnerID *uint `json:",omitempty" gorm:"uniqueIndex:idx_owner_ref"`
	Owner   *User `gorm:"foreignKey:OwnerID" json:"-"`

	SearchConfidenceImage *float64 `gorm:"-" json:",omitempty"`
	SearchConfidenceText  *float64 `gorm:"-" json:",omitempty"`
}

func (i *RecordInput) Convert() (o Record, err error) {
	o.Quantity = i.Quantity
	o.ReferenceNumber = i.ReferenceNumber
	o.Title = i.Title
	o.Description = i.Description

	if i.ParentID != nil {
		var found []Record
		found, err = gorm.G[Record](db).Where("id = ?", *i.ParentID).Find(dbCtx)
		if err != nil {
			return
		} else if len(found) > 1 {
			err = huma.Error500InternalServerError(errorMoreRecordsThanExpected)
			return
		} else if len(found) == 0 {
			err = huma.Error404NotFound(errorRecordNotFound + " " + strconv.Itoa(int(*i.ParentID)))
			return
		}
		o.ParentID = i.ParentID
	}

	for _, artifact := range i.Artifacts {
		var foundArtifact Artifact
		foundArtifact, err = GetArtifactFromDB(*artifact)
		if err != nil {
			return
		}
		o.Artifacts = append(o.Artifacts, &foundArtifact)
	}

	return

}

func (record *Record) PrettyString() (output string) {
	output = strconv.FormatUint(uint64(record.ID), 10)
	if record.ReferenceNumber != nil && *record.ReferenceNumber != "" {
		output += " (" + *record.ReferenceNumber + ")"
	}
	return
}

// GetRecordEmbeddings returns text embeddings for the given record IDs.
// Only jobs for those specific IDs are enqueued and waited on, so partial
// reflects completeness within the requested scope only.
func GetRecordEmbeddings(ctx context.Context, scopedIDs []uint) (e map[uint][]float64, partial bool, err error) {
	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, _, _ := effectiveInfinityConfig(user)

	if len(scopedIDs) == 0 {
		return map[uint][]float64{}, false, nil
	}

	var embeddings []Embedding
	if err = db.Where("record_id IN ? AND embed_model = ?", scopedIDs, textModel).Find(&embeddings).Error; err != nil {
		return
	}

	e = map[uint][]float64{}
	embeddedIDs := make(map[uint]bool, len(embeddings))

	for _, emb := range embeddings {
		if emb.RecordID == nil {
			continue
		}
		var vec []float64
		if cached, ok := embeddingsCache.Load(emb.Hash); ok {
			vec = cached.(Embeddings)
		} else {
			if err = json.Unmarshal(emb.Data, &vec); err != nil {
				return
			}
			embeddingsCache.Store(emb.Hash, Embeddings(vec))
		}
		e[*emb.RecordID] = vec
		embeddedIDs[*emb.RecordID] = true
	}

	missingIDs := make([]uint, 0, len(scopedIDs))
	for _, id := range scopedIDs {
		if !embeddedIDs[id] {
			missingIDs = append(missingIDs, id)
		}
	}

	enqueuedIDs, err := generateMissingRecordEmbeddings(ctx, missingIDs, nil, "search")
	if err != nil {
		return
	}
	partial = len(enqueuedIDs) > 0

	return
}

// generateMissingRecordEmbeddings enqueues embedding jobs for record IDs not in embeddedIDs.
// Returns the IDs that were enqueued.
func generateMissingRecordEmbeddings(ctx context.Context, recordIDs []uint, embeddedIDs map[uint]bool, source string) (enqueued []uint, err error) {
	username, user, userID, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, _, _ := effectiveInfinityConfig(user)

	for _, id := range recordIDs {
		if !embeddedIDs[id] {
			EnqueueEmbeddingJob(JobTypeRecord, id, userID, username, textModel, source)
			enqueued = append(enqueued, id)
		}
	}
	Log.Infow("generateMissingRecordEmbeddings", "source", source, "username", username, "total", len(recordIDs), "enqueued", len(enqueued))
	return
}

func recordEmbeddingText(r Record) string {
	parts := []string{}
	if r.Title != nil && *r.Title != "" {
		parts = append(parts, *r.Title)
	}
	if r.ReferenceNumber != nil && *r.ReferenceNumber != "" {
		parts = append(parts, *r.ReferenceNumber)
	}
	if r.Description != nil && *r.Description != "" {
		parts = append(parts, *r.Description)
	}
	return strings.Join(parts, " - ")
}

func (r *Record) GenerateEmbeddings(ctx context.Context) (vec Embeddings, err error) {
	text := recordEmbeddingText(*r)
	if text == "" {
		return
	}

	_, user, _, err := UserFromContext(ctx)
	if err != nil {
		return
	}
	textModel, _, _, _ := effectiveInfinityConfig(user)

	var fullInput string
	vec, fullInput, err = GenerateTextDocumentEmbeddingsCtx(ctx, text)
	if err != nil {
		return
	}

	id := r.ID
	err = saveEmbedding(&id, nil, vec, textModel, fullInput)
	if err == nil {
		Log.Infof("embedding: record %d indexed with model %s", id, textModel)
	}
	return
}
