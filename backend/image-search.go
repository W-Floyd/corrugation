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
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

// ImageSearchInput represents the input for the POST /api/v2/search/image endpoint
type ImageSearchInput struct {
	Body struct {
		File huma.FormFile `form:"file" required:"true" doc:"Image file to search for similar records"`
	}
}

// ImageSearchOutput represents the output for the image search endpoint
type ImageSearchOutput struct {
	Status int `yaml:"-"`
	Body   []struct {
		RecordID   uint    `json:"recordId"`
		Confidence float64 `json:"confidence"`
	} `json:"results"`
}

// SearchByImageOperation defines the POST /api/v2/search/image endpoint for image-based record search
var SearchByImageOperation = huma.Operation{
	Method:      "POST",
	Path:        "/api/v2/search/image",
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
func SearchByImageHandler(ctx context.Context, input *ImageSearchInput) (output *ImageSearchOutput, err error) {
	// Read the uploaded image file
	file := input.Body.File
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

	// Convert to ImageSearchOutput format
	outputResults := make([]struct {
		RecordID   uint    `json:"recordId"`
		Confidence float64 `json:"confidence"`
	}, len(results))
	for i, result := range results {
		outputResults[i].RecordID = result.ID
		if result.SearchConfidenceImage != nil {
			outputResults[i].Confidence = *result.SearchConfidenceImage
		}
	}

	output = &ImageSearchOutput{
		Status: http.StatusOK,
		Body:   outputResults,
	}

	return output, nil
}

// SearchByImage searches for records using image embedding similarity
// Returns records sorted by confidence score, filtered by minimumImageSearchConfidence threshold
func SearchByImage(ctx context.Context, imageData []byte) (results []struct {
	RecordID   uint    `json:"recordId"`
	Confidence float64 `json:"confidence"`
}, err error) {
	// Get current user
	username := UsernameFromContext(ctx)
	var userID *uint
	if username != "" {
		var user User
		user, err = loadUser(username)
		if err != nil {
			return nil, err
		}
		userID = &user.ID
	}

	// Generate image embedding from uploaded image
	imageEmbeddings, err := generateImageEmbeddingFromBytes(imageData)
	if err != nil {
		return nil, err
	}

	// Get record image embeddings from database, scoped by user
	var embeddings []Embedding
	if userID != nil {
		// First get the record IDs we're allowed to search
		var allowedRecordIDs []uint
		if err = db.Model(&Record{}).Where("owner_id = ?", *userID).Pluck("id", &allowedRecordIDs).Error; err != nil {
			return nil, err
		}

		// Get embeddings for allowed records
		if err = db.Where("record_id IS NOT NULL AND record_id IN ? AND embed_model = ?", allowedRecordIDs, infinityImageModel).Find(&embeddings).Error; err != nil {
			return nil, err
		}
	} else {
		// Anonymous user: search all records
		if err = db.Where("record_id IS NOT NULL AND embed_model = ?", infinityImageModel).Find(&embeddings).Error; err != nil {
			return nil, err
		}
	}

	// Compute similarity scores for all embeddings
	type recordScore struct {
		recordID uint
		score    float64
	}
	scores := make([]recordScore, 0)

	for _, emb := range embeddings {
		if emb.RecordID == nil {
			continue
		}
		var recordVec []float64
		if cached, ok := embeddingsCache.Load(emb.Hash); ok {
			recordVec = cached.(Embeddings)
		} else {
			if err = json.Unmarshal(emb.Data, &recordVec); err != nil {
				continue
			}
			embeddingsCache.Store(emb.Hash, Embeddings(recordVec))
		}
		score, err := dotProduct(recordVec, imageEmbeddings)
		if err != nil {
			continue
		}
		scores = append(scores, recordScore{recordID: *emb.RecordID, score: score})
	}

	// Filter by minimum confidence threshold
	var filtered []struct {
		RecordID   uint    `json:"recordId"`
		Confidence float64 `json:"confidence"`
	}
	for _, s := range scores {
		if s.score >= minimumImageSearchConfidence {
			filtered = append(filtered, struct {
				RecordID   uint    `json:"recordId"`
				Confidence float64 `json:"confidence"`
			}{RecordID: s.recordID, Confidence: s.score})
		}
	}

	// Sort by confidence score (descending)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	return filtered, nil
}

// generateImageEmbeddingFromBytes creates an image embedding from raw image bytes
func generateImageEmbeddingFromBytes(imageData []byte) ([]float64, error) {
	// Decode the image to validate it
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil || img == nil {
		return nil, errors.New("invalid image data")
	}

	// Create base64-encoded image for Infinity
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	base64Image = "data:image/jpeg;base64," + base64Image

	// Generate embedding using Infinity
	request := infinityEmbeddingsRequest{
		Model:          infinityImageModel,
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
func GetRecordsWithImageSimilarity(ctx context.Context, imageData []byte) ([]RecordResponse, error) {
	scores, err := SearchByImage(ctx, imageData)
	if err != nil {
		return nil, err
	}

	if len(scores) == 0 {
		return []RecordResponse{}, nil
	}

	recordIDs := make([]uint, len(scores))
	for i, s := range scores {
		recordIDs[i] = s.RecordID
	}

	// Fetch records from database, scoped by user
	username := UsernameFromContext(ctx)
	var userID *uint
	if username != "" {
		var user User
		user, err = loadUser(username)
		if err != nil {
			return nil, err
		}
		userID = &user.ID
	}

	var records []Record
	if userID != nil {
		// Only fetch records owned by the user
		records, err = gorm.G[Record](db).Where("id IN ? AND owner_id = ?", recordIDs, *userID).Find(dbCtx)
		if err != nil {
			return nil, err
		}
	} else {
		// Anonymous user: fetch all records
		records, err = gorm.G[Record](db).Where("id IN ?", recordIDs).Find(dbCtx)
		if err != nil {
			return nil, err
		}
	}

	// Build a map of record IDs to scores
	scoreMap := make(map[uint]float64)
	for _, s := range scores {
		scoreMap[s.RecordID] = s.Confidence
	}

	// Build responses with similarity scores
	var responses []RecordResponse
	for _, record := range records {
		score := scoreMap[record.ID]
		record.SearchConfidenceImage = &score
		responses = append(responses, toRecordResponse(record, true))
	}

	return responses, nil
}
