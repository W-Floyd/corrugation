package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"io"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
	"github.com/labstack/gommon/log"
	"gorm.io/gorm"
)

// ImageSearchInput represents the input for the POST /api/search/image endpoint
type ImageSearchInput struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true"`
	}]
}

// SearchByImageOperation defines the POST /api/search/image endpoint for image-based record search
var SearchByImageOperation = huma.Operation{
	Method:      "POST",
	Path:        "/api/search/image",
	Summary:     "Upload an image and search for similar records",
	Description: `Accepts an image file via multipart form and searches the database for records with visually similar embeddings using the CLIP model. Returns matching records sorted by confidence score (dot product similarity). Images are processed through Infinity's embedding service and compared against existing record image embeddings.`,
	Responses: map[string]*huma.Response{
		"200": {
			Description: "Successful search results",
			Content:     map[string]*huma.MediaType{"application/json": {}},
		},
		"400": {
			Description: "Invalid image file or upload error",
			Content:     map[string]*huma.MediaType{"application/json": {}},
		},
		"500": {
			Description: "Server error during embedding or search",
			Content:     map[string]*huma.MediaType{"application/json": {}},
		},
	},
}

// SearchByImageHandler handles image upload and similarity search
func SearchByImageHandler(ctx context.Context, input *ImageSearchInput) (output *RecordsOutput, err error) {
	// Read the uploaded image file
	file := input.RawBody.Data().File
	imageData, err := io.ReadAll(file)
	if err != nil {
		return nil, huma.Error400BadRequest("failed to read image file", err)
	}

	// Validate image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil || img == nil {
		return nil, huma.Error400BadRequest("invalid image file", err)
	}

	// Perform the image search
	results, err := GetRecordsWithImageSimilarity(ctx, imageData)
	if err != nil {
		return nil, huma.Error500InternalServerError("search failed", err)
	}

	output = &RecordsOutput{
		Status: http.StatusOK,
		Body:   results,
	}

	return output, nil
}

// generateImageEmbeddingFromBytes creates an image embedding from raw image bytes
func generateImageEmbeddingFromBytes(user *User, imageData []byte) ([]float64, error) {
	// Decode the image to validate it
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil || img == nil {
		return nil, errors.New("invalid image data")
	}

	// Create base64-encoded image for Infinity
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	base64Image = "data:image/jpeg;base64," + base64Image

	_, imageModel, _, _ := effectiveInfinityConfig(user)
	request := infinityEmbeddingsRequest{
		Model:          imageModel,
		EncodingFormat: "float",
		Input:          []string{base64Image},
		Modality:       "image",
	}

	embedding, err := request.GenerateEmbeddings()
	if err != nil {
		return nil, err
	}

	return embedding, nil
}

// GetRecordsWithImageSimilarity retrieves records with their image similarity scores
// Returns a list of RecordResponse objects with SearchConfidenceImage set
func GetRecordsWithImageSimilarity(ctx context.Context, imageData []byte) (results []RecordResponse, err error) {
	// Get current user
	_, user, userID, err := UserFromContext(ctx)
	if err != nil {
		log.Error(err)
		return
	}

	// Generate image embedding from uploaded image
	imageEmbeddings, err := generateImageEmbeddingFromBytes(user, imageData)
	if err != nil {
		log.Error(err)
		return
	}
	// Get record image embeddings from database, scoped by user
	var recordEmbeddings []struct {
		Record        `gorm:"embedded"`
		EmbeddingData *[]byte `gorm:"column:embedding_data"`
		EmbeddingHash *string `gorm:"column:embedding_hash"`
	}

	_, imageModel, _, _ := effectiveInfinityConfig(user)

	q := db.Table("embeddings").
		Select("records.*, embeddings.data as embedding_data, embeddings.hash as embedding_hash").
		Joins("JOIN artifacts ON artifacts.id = embeddings.artifact_id").
		Joins("JOIN records ON records.id = artifacts.record_id").
		Where("embeddings.embed_model = ?", imageModel)

	if userID != nil {
		q = q.Where("records.owner_id = ?", userID)
	}

	if err = q.Find(&recordEmbeddings).Error; err != nil {
		log.Error(err)
		return
	}

	ids := []uint{}

	for _, r := range recordEmbeddings {
		ids = append(ids, r.Record.ID)
	}

	records := []Record{}
	records, err = gorm.G[Record](db).Where("id IN ?", ids).
		Preload("Artifacts",
			func(db gorm.PreloadBuilder) error {
				db.Select("id", "record_id")
				return nil
			},
		).Find(dbCtx)
	if err != nil {
		log.Error(err)
		return
	}

	recordMap := map[uint]*Record{}

	for _, r := range records {
		recordMap[r.ID] = &r
	}

	for _, r := range recordEmbeddings {
		var recordVec []float64
		if cached, ok := embeddingsCache.Load(*r.EmbeddingHash); ok {
			recordVec = cached.(Embeddings)
		} else {
			if err = json.Unmarshal(*r.EmbeddingData, &recordVec); err != nil {
				log.Error(err)
				return
			}
			embeddingsCache.Store(*r.EmbeddingHash, Embeddings(recordVec))
		}
		var score float64
		score, err = dotProduct(recordVec, imageEmbeddings)
		if err != nil {
			log.Error(err)
			return
		}
		if score < minimumImageToImageSearchConfidence {
			continue
		}
		record := toRecordResponse(*recordMap[r.ID], false)
		record.SearchConfidenceImage = &score
		results = append(results, record)
	}

	slices.SortFunc(results, func(a, b RecordResponse) int {
		if a.SearchConfidenceImage == nil {
			return 1
		}
		if b.SearchConfidenceImage == nil {
			return -1
		}
		if *a.SearchConfidenceImage > *b.SearchConfidenceImage {
			return -1
		}
		if *a.SearchConfidenceImage < *b.SearchConfidenceImage {
			return 1
		}
		return 0
	})

	return
}
