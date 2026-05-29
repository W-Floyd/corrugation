package backend

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type ollamaGenerateRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Images []string `json:"images"`
	Stream bool     `json:"stream"`
	Format string   `json:"format"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// ItemSuggestions holds content suggested from an image.
type ItemSuggestions struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Quantity    *uint  `json:"quantity,omitempty"`
}

// ArtifactSuggestion is the persisted cache of an Ollama suggestion for an artifact.
type ArtifactSuggestion struct {
	Model
	ArtifactID  uint   `gorm:"not null;uniqueIndex:idx_artifact_suggestion"`
	OllamaModel string `gorm:"not null;uniqueIndex:idx_artifact_suggestion"`
	Name        string
	Description string
	Quantity    *uint
}

func saveSuggestion(artifactID uint, model string, s ItemSuggestions) error {
	var existing ArtifactSuggestion
	err := db.Where("artifact_id = ? AND ollama_model = ?", artifactID, model).First(&existing).Error
	if err == nil {
		return db.Model(&existing).Updates(map[string]interface{}{
			"name":        s.Name,
			"description": s.Description,
			"quantity":    s.Quantity,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&ArtifactSuggestion{
		ArtifactID:  artifactID,
		OllamaModel: model,
		Name:        s.Name,
		Description: s.Description,
		Quantity:    s.Quantity,
	}).Error
}

// GetArtifactSuggestion returns the cached suggestion for an artifact+model, or nil if not found.
func GetArtifactSuggestion(artifactID uint, model string) (*ArtifactSuggestion, error) {
	var s ArtifactSuggestion
	err := db.Where("artifact_id = ? AND ollama_model = ?", artifactID, model).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

const suggestPrompt = `You are analyzing a household inventory item photo. Return a JSON object with these fields:
- "name": a short, descriptive name for the item (string)
- "description": a brief description of the item including notable features (string)
- "quantity": estimated visible quantity as a whole number, or null if unclear (number or null)

Respond with valid JSON only. No explanation, no markdown.`

func generateItemSuggestions(address, model string, imageData []byte) (ItemSuggestions, error) {
	b64 := base64.StdEncoding.EncodeToString(imageData)

	reqBody := ollamaGenerateRequest{
		Model:  model,
		Prompt: suggestPrompt,
		Images: []string{b64},
		Stream: false,
		Format: "json",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ItemSuggestions{}, err
	}

	resp, err := http.Post(address+"/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return ItemSuggestions{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ItemSuggestions{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return ItemSuggestions{}, errors.Join(
			errors.New(string(respBody)),
			errors.New("http error "+strconv.Itoa(resp.StatusCode)+" from Ollama"),
		)
	}

	var ollamaResp ollamaGenerateResponse
	if err = json.Unmarshal(respBody, &ollamaResp); err != nil {
		return ItemSuggestions{}, err
	}

	var suggestions ItemSuggestions
	if err = json.Unmarshal([]byte(ollamaResp.Response), &suggestions); err != nil {
		return ItemSuggestions{}, errors.New("ollama returned non-JSON response: " + ollamaResp.Response)
	}

	return suggestions, nil
}

// effectiveOllamaConfig returns the active Ollama address and vision model,
// preferring GlobalConfig values over the CLI-seeded constants.
func effectiveOllamaConfig() (address, visionModel string) {
	cfg, err := loadGlobalConfig()
	if err == nil && cfg.OllamaAddress != "" {
		address = cfg.OllamaAddress
	} else {
		address = ollamaAddress
	}
	if err == nil && cfg.OllamaVisionModel != "" {
		visionModel = cfg.OllamaVisionModel
	} else {
		visionModel = ollamaVisionModel
	}
	return
}

// --- Suggest endpoint ---

var SuggestFromImageOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/suggest",
	DefaultStatus: http.StatusOK,
	OperationID:   "suggest-from-image",
}

func SuggestFromImage(_ context.Context, input *struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true"`
	}]
}) (output *struct{ Body ItemSuggestions }, err error) {
	addr, model := effectiveOllamaConfig()
	if addr == "" {
		err = huma.Error503ServiceUnavailable("ollama not configured")
		return
	}

	f := input.RawBody.Data().File
	data, readErr := io.ReadAll(f.File)
	if readErr != nil {
		err = huma.Error400BadRequest("failed to read file")
		return
	}

	suggestions, suggestErr := generateItemSuggestions(addr, model, data)
	if suggestErr != nil {
		Log.Errorw("ollama suggest failed", "error", suggestErr)
		err = huma.Error503ServiceUnavailable("suggestion failed: " + suggestErr.Error())
		return
	}

	output = &struct{ Body ItemSuggestions }{Body: suggestions}
	return
}
