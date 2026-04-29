package backend

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type ListRecordsInput struct {
	ID                  int     `query:"id" example:"1" doc:"Record ID; 0 = top-level (parent IS NULL); -1 = omitted (use global flag for all)" required:"false" default:"-1"`
	Global              bool    `query:"global" doc:"Return all records regardless of location" required:"false"`
	ChildrenDepth       int     `query:"childrenDepth" example:"2" doc:"Depth to search for children, negative values mean unlimited search" required:"false"`
	ParentDepth         int     `query:"parentDepth" example:"2" doc:"Depth to search for parents, negative values mean unlimited search" required:"false"`
	Search              string  `query:"search" example:"Lamp" doc:"String to search embeddings with" required:"false"`
	SearchImage         bool    `query:"searchImage" doc:"Use image embeddings in search" required:"false"`
	SearchTextEmbedded  bool    `query:"searchTextEmbedded" doc:"Use text embeddings in search" required:"false"`
	SearchTextSubstring bool    `query:"searchTextSubstring" doc:"Use substring matching in search" required:"false"`
	MinImageScore       float64 `query:"minImageScore" doc:"Minimum image embedding score threshold" required:"false"`
	MinTextScore        float64 `query:"minTextScore" doc:"Minimum text score threshold" required:"false"`
	Timestamps          bool    `query:"timestamps" doc:"Include CreatedAt and UpdatedAt in response" required:"false"`
}

type RecordOutput struct {
	Body RecordResponse
}

type RecordsOutput struct {
	Status int `yaml:"-"`
	Body   []RecordResponse
}

var GetRecordOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/v2/record/{id}",
}

func GetRecord(ctx context.Context, input *struct {
	ID         uint `path:"id" example:"1" doc:"ID to delete"`
	Timestamps bool `query:"timestamps" doc:"Include CreatedAt and UpdatedAt in response" required:"false"`
}) (output *RecordOutput, err error) {
	var records []Record
	records, _, err = GetRecords(ctx, &input.ID, nil, nil, nil, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{
			q: "Artifacts",
			h: func(db gorm.PreloadBuilder) error {
				db.Select("id", "record_id")
				return nil
			},
		},
	}, nil)
	if err != nil {
		return
	}
	output = &RecordOutput{
		Body: toRecordResponse(records[0], input.Timestamps),
	}
	return
}

var ListRecordsOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/v2/records",
}

func ListRecords(ctx context.Context, input *ListRecordsInput) (output *RecordsOutput, err error) {
	var records []Record
	s := NewRecordQuery(input.Search)
	search := &s
	s.SearchImage = input.SearchImage
	s.SearchTextEmbedded = input.SearchTextEmbedded
	s.SearchTextSubstring = input.SearchTextSubstring
	s.ChildrenDepth = input.ChildrenDepth
	s.ParentDepth = input.ParentDepth
	if input.MinImageScore > 0 {
		s.MinImageScore = input.MinImageScore
	}
	if input.MinTextScore > 0 {
		s.MinTextScore = input.MinTextScore
	}
	var ID *uint
	if !input.Global {
		if input.ID >= 0 {
			v := uint(input.ID)
			ID = &v
		} else {
			var zero uint = 0
			ID = &zero
		}
	}
	var childrenDepth, parentDepth *int
	if s.ChildrenDepth != 0 {
		childrenDepth = &s.ChildrenDepth
	}
	if s.ParentDepth != 0 {
		parentDepth = &s.ParentDepth
	}

	var partial bool
	records, partial, err = GetRecords(ctx, ID, childrenDepth, parentDepth, search, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{q: "Artifacts", h: func(db gorm.PreloadBuilder) error { db.Select("id", "record_id"); return nil }},
	}, nil)
	if err != nil {
		return
	}
	responses := make([]RecordResponse, len(records))
	for i, r := range records {
		responses[i] = toRecordResponse(r, input.Timestamps)
	}
	status := http.StatusOK
	if partial {
		status = http.StatusMultiStatus
	}
	output = &RecordsOutput{Status: status, Body: responses}
	return
}

var CreateRecordOperation = huma.Operation{
	Method: http.MethodPost,
	Path:   "/api/v2/record",
}

func checkReferenceNumberAvailable(refNum string, ownerID *uint, excludeID *uint) error {
	var existing Record
	q := db.Where("reference_number = ?", refNum).Where("owner_id = ?", ownerID)
	if excludeID != nil {
		q = q.Where("id != ?", *excludeID)
	}
	if err := q.First(&existing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	return huma.Error409Conflict("reference number is already in use")
}

func CreateRecord(ctx context.Context, input *struct {
	Body RecordInput
}) (output *RecordOutput, err error) {
	username := UsernameFromContext(ctx)
	var userID *uint
	if username != "" {
		var user User
		user, err = loadUser(username)
		if err != nil {
			return
		}
		userID = &user.ID
	}

	if input.Body.ReferenceNumber != nil {
		if err = checkReferenceNumberAvailable(*input.Body.ReferenceNumber, userID, nil); err != nil {
			return
		}
	}

	record, err := input.Body.Convert()
	if err != nil {
		return
	}
	record.OwnerID = userID

	err = gorm.G[Record](db).Create(dbCtx, &record)
	if err != nil {
		return
	}
	uc, _ := loadUser(username)
	textModel, _, _, _ := effectiveInfinityConfig(uc)
	EnqueueEmbeddingJob(JobTypeRecord, record.ID, userID, username, textModel, "store")
	err = nil
	output = &RecordOutput{
		Body: toRecordResponse(record, true),
	}
	return
}

var UpdateRecordOperation = huma.Operation{
	Method: http.MethodPost,
	Path:   "/api/v2/record/{id}",
}

func UpdateRecord(ctx context.Context, input *struct {
	ID   uint `path:"id"`
	Body RecordInput
}) (output *RecordOutput, err error) {
	records, _, err := GetRecords(ctx, &input.ID, nil, nil, nil, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{q: "Artifacts", h: func(db gorm.PreloadBuilder) error { db.Select("id", "record_id"); return nil }},
		{q: "Tags", h: func(db gorm.PreloadBuilder) error { return nil }},
	}, nil)
	if err != nil {
		return
	}

	r := records[0]

	if input.Body.ReferenceNumber != nil {
		if err = checkReferenceNumberAvailable(*input.Body.ReferenceNumber, r.OwnerID, &r.ID); err != nil {
			return
		}
	}

	updated, err := input.Body.Convert()
	if err != nil {
		return
	}

	if updated.Artifacts != nil {
		r.Artifacts = updated.Artifacts
	}

	if input.Body.Tags != nil {
		if err = db.Model(&r).Association("Tags").Replace(updated.Tags); err != nil {
			return
		}
		r.Tags = updated.Tags
	}

	err = db.Model(&r).Updates(map[string]any{
		"quantity":         updated.Quantity,
		"reference_number": updated.ReferenceNumber,
		"title":            updated.Title,
		"description":      updated.Description,
		"parent_id":        updated.ParentID,
	}).Error
	if err != nil {
		return
	}

	updateUsername := UsernameFromContext(ctx)
	updateUC, _ := loadUser(updateUsername)
	textModel, _, _, _ := effectiveInfinityConfig(updateUC)
	var updateOwnerID *uint
	if updateUC.ID > 0 {
		updateOwnerID = &updateUC.ID
	}
	EnqueueEmbeddingJob(JobTypeRecord, r.ID, updateOwnerID, updateUsername, textModel, "store")

	output = &RecordOutput{Body: toRecordResponse(r, true)}
	return
}

var GetNextReferenceNumberOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/v2/records/nextref",
}

func GetNextReferenceNumber(ctx context.Context, input *struct {
	ExcludeIDs []uint `query:"excludeIDs"`
}) (output *UIntOutput, err error) {
	var refs []string
	q := db.Model(&Record{}).Where("reference_number NOT NULL")
	if len(input.ExcludeIDs) > 0 {
		q = q.Not("id IN ?", input.ExcludeIDs)
	}
	q = q.Order("CAST(reference_number AS unsigned)")
	if username := UsernameFromContext(ctx); username != "" {
		var user User
		if user, err = loadUser(username); err != nil {
			return
		}
		q = q.Where("owner_id = ?", user.ID)
	}
	if tx := q.Pluck("reference_number", &refs); tx.Error != nil {
		err = tx.Error
		return
	}

	nums := []int{}

	for _, ref := range refs {
		v, err := strconv.Atoi(strings.TrimSpace(ref))
		if err == nil {
			nums = append(nums, v)
		}
	}

	if len(nums) == 0 {
		output = &UIntOutput{Body: 1}
		return
	}

	sort.Ints(nums)

	low := 1
	for _, n := range nums {
		if n == low {
			low++
		} else if n > low {
			break
		}
	}

	output = &UIntOutput{Body: uint(low)}
	return
}

var DeleteRecordOperation = huma.Operation{
	Method: http.MethodDelete,
	Path:   "/api/v2/record/{id}",
}

func DeleteRecord(ctx context.Context, input *struct {
	ID uint `path:"id" example:"1" doc:"ID to delete"`
}) (output *EmptyOutput, err error) {
	username := UsernameFromContext(ctx)
	chain := gorm.G[Record](db).Where("id = ?", input.ID)
	if username != "" {
		var user User
		user, err = loadUser(username)
		if err != nil {
			return
		}
		chain = gorm.G[Record](db).Where("id = ? AND owner_id = ?", input.ID, user.ID)
	}

	records, err := chain.Find(dbCtx)
	if err != nil {
		return
	}
	if len(records) == 0 {
		err = huma.Error404NotFound(errorRecordNotFound + " " + strconv.Itoa(int(input.ID)))
		return
	} else if len(records) > 1 {
		err = huma.Error500InternalServerError(errorMoreRecordsThanExpected)
		return
	}

	newParentID := records[0].ParentID

	// Children inherit parentID if available
	if newParentID != nil {
		_, err = gorm.G[Record](db).Where("parent_id = ?", input.ID).Update(dbCtx, "parent_id", *newParentID)
		if err != nil {
			return
		}
	} else {
		_, err = gorm.G[Record](db).Where("parent_id = ?", input.ID).Update(dbCtx, "parent_id", nil)
		if err != nil {
			return
		}
	}

	tx := db.Unscoped().Where("id = ?", input.ID).Delete(&Record{})
	if tx.Error != nil {
		err = tx.Error
		return
	}
	if tx.RowsAffected == 0 {
		err = huma.Error404NotFound(errorRecordNotFound + " " + strconv.Itoa(int(input.ID)))
	}
	output = &EmptyOutput{}
	return
}

var FlushStaleEmbeddingsOperation = huma.Operation{
	Method: http.MethodPost,
	Path:   "/api/v2/embeddings/flush",
}

func FlushStaleEmbeddings(ctx context.Context, _ *struct{}) (output *struct {
	Body struct {
		RecordsFlushed   int64 `json:"recordsFlushed"`
		ArtifactsFlushed int64 `json:"artifactsFlushed"`
	}
}, err error) {
	stale := "embed_model != ? AND embed_model != ?"

	recordsFlushed, err := gorm.G[Embedding](db).Where("record_id IS NOT NULL AND "+stale, infinityTextModel, infinityImageModel).Delete(dbCtx)
	if err != nil {
		return
	}
	artifactsFlushed, err := gorm.G[Embedding](db).Where("artifact_id IS NOT NULL AND "+stale, infinityTextModel, infinityImageModel).Delete(dbCtx)
	if err != nil {
		return
	}

	output = &struct {
		Body struct {
			RecordsFlushed   int64 `json:"recordsFlushed"`
			ArtifactsFlushed int64 `json:"artifactsFlushed"`
		}
	}{Body: struct {
		RecordsFlushed   int64 `json:"recordsFlushed"`
		ArtifactsFlushed int64 `json:"artifactsFlushed"`
	}{
		RecordsFlushed:   int64(recordsFlushed),
		ArtifactsFlushed: int64(artifactsFlushed),
	}}
	return
}

var VisualizeGraphRecordsOperation = huma.Operation{
	Method: http.MethodGet,
	Path:   "/api/v2/records/visualize",
}

func VisualizeGraphRecords(ctx context.Context, input *ListRecordsInput) (output *BytesOutput, err error) {

	var graph string
	var visID uint
	if input.ID > 0 {
		visID = uint(input.ID)
	}
	graph, err = GetRecordsGraphFriendlyNative(ctx, visID, input.ChildrenDepth, input.ParentDepth)
	if err != nil {
		return
	}

	output = &BytesOutput{ContentType: "text/html",
		Body: []byte(graph),
	}
	return
}
