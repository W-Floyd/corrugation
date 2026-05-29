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
	"time"

	"github.com/danielgtaylor/huma/v2"
	"gorm.io/gorm"
)

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Images  []string       `json:"images"`
	Stream  bool           `json:"stream"`
	Format  string         `json:"format"`
	Options map[string]any `json:"options,omitempty"`
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

func generateItemSuggestions(address, model string, numCtx int, imageData []byte) (ItemSuggestions, error) {
	b64 := base64.StdEncoding.EncodeToString(imageData)

	reqBody := ollamaGenerateRequest{
		Model:   model,
		Prompt:  suggestPrompt,
		Images:  []string{b64},
		Stream:  false,
		Format:  "json",
		Options: map[string]any{"num_ctx": numCtx},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return ItemSuggestions{}, err
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(address+"/api/generate", "application/json", bytes.NewBuffer(body))
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

	if ollamaResp.Response == "" {
		return ItemSuggestions{}, errors.New("ollama returned an empty response — model may not support vision or JSON output")
	}
	var suggestions ItemSuggestions
	if err = json.Unmarshal([]byte(ollamaResp.Response), &suggestions); err != nil {
		return ItemSuggestions{}, errors.New("ollama returned non-JSON response: " + ollamaResp.Response)
	}

	return suggestions, nil
}

// effectiveOllamaConfig returns the active Ollama address, vision model, and num_ctx.
// Priority: per-user override → GlobalConfig → CLI-seeded constants.
func effectiveOllamaConfig(u ...*User) (address, visionModel string, numCtx int) {
	address = ollamaAddress
	visionModel = ollamaVisionModel
	numCtx = ollamaNumCtx

	if cfg, err := loadGlobalConfig(); err == nil {
		if cfg.OllamaAddress != "" {
			address = cfg.OllamaAddress
		}
		if cfg.OllamaVisionModel != "" {
			visionModel = cfg.OllamaVisionModel
		}
		if cfg.OllamaNumCtx > 0 {
			numCtx = cfg.OllamaNumCtx
		}
	}

	var user *User
	if len(u) > 0 {
		user = u[0]
	}
	if user != nil {
		if user.OllamaAddress != nil {
			address = *user.OllamaAddress
		}
		if user.OllamaVisionModel != nil {
			visionModel = *user.OllamaVisionModel
		}
		if user.OllamaNumCtx != nil {
			numCtx = *user.OllamaNumCtx
		}
	}
	return
}

// EnsureOllamaModel checks whether the configured vision model is available
// and pulls it in the background if not. Safe to call at startup.
func EnsureOllamaModel() {
	addr, model, _ := effectiveOllamaConfig()
	if addr == "" || model == "" {
		return
	}

	resp, err := http.Get(addr + "/api/tags")
	if err != nil {
		Log.Infow("ollama: could not check models at startup", "error", err)
		return
	}
	defer resp.Body.Close()

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return
	}

	for _, m := range tagsResp.Models {
		if m.Name == model {
			Log.Infow("ollama: vision model already available", "model", model)
			return
		}
	}

	Log.Infow("ollama: vision model not found, pulling", "model", model)
	body, _ := json.Marshal(map[string]any{"name": model, "stream": false})
	client := &http.Client{Timeout: 30 * time.Minute}
	pullResp, err := client.Post(addr+"/api/pull", "application/json", bytes.NewBuffer(body))
	if err != nil {
		Log.Errorw("ollama: pull failed", "model", model, "error", err)
		BroadcastAll("ollama_pull_failed:" + model)
		return
	}
	defer pullResp.Body.Close()
	if pullResp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(pullResp.Body)
		Log.Errorw("ollama: pull error response", "model", model, "status", pullResp.StatusCode, "body", string(msg))
		BroadcastAll("ollama_pull_failed:" + model)
		return
	}
	Log.Infow("ollama: pull complete", "model", model)
	BroadcastAll("ollama_pull_complete:" + model)
}

// --- List models endpoint ---

var ListOllamaModelsOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/ollama/models",
	DefaultStatus: http.StatusOK,
	OperationID:   "list-ollama-models",
}

func ListOllamaModels(_ context.Context, _ *struct{}) (output *struct{ Body []string }, err error) {
	addr, _, _ := effectiveOllamaConfig()
	if addr == "" {
		output = &struct{ Body []string }{Body: []string{}}
		return
	}

	resp, err := http.Get(addr + "/api/tags")
	if err != nil {
		err = huma.Error503ServiceUnavailable("ollama not reachable: " + err.Error())
		return
	}
	defer resp.Body.Close()

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		err = huma.Error503ServiceUnavailable("failed to parse ollama response")
		return
	}

	names := make([]string, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		names[i] = m.Name
	}
	output = &struct{ Body []string }{Body: names}
	return
}

// --- Pull model endpoint ---

var PullOllamaModelOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/ollama/pull",
	DefaultStatus: http.StatusAccepted,
	OperationID:   "pull-ollama-model",
}

func PullOllamaModel(_ context.Context, input *struct {
	Body struct {
		Model string `json:"model" doc:"Model name to pull"`
	}
}) (*struct{}, error) {
	addr, _, _ := effectiveOllamaConfig()
	if addr == "" {
		return nil, huma.Error503ServiceUnavailable("ollama not configured")
	}
	if input.Body.Model == "" {
		return nil, huma.Error400BadRequest("model name is required")
	}

	model := input.Body.Model
	go func() {
		body, _ := json.Marshal(map[string]any{"name": model, "stream": false})
		client := &http.Client{Timeout: 30 * time.Minute}
		resp, err := client.Post(addr+"/api/pull", "application/json", bytes.NewBuffer(body))
		if err != nil {
			Log.Errorw("ollama pull failed", "model", model, "error", err)
			BroadcastAll("ollama_pull_failed:" + model)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			msg, _ := io.ReadAll(resp.Body)
			Log.Errorw("ollama pull error response", "model", model, "status", resp.StatusCode, "body", string(msg))
			BroadcastAll("ollama_pull_failed:" + model)
			return
		}
		Log.Infow("ollama pull complete", "model", model)
		BroadcastAll("ollama_pull_complete:" + model)
	}()

	return nil, nil
}

// --- Suggest endpoint ---

var SuggestFromImageOperation = huma.Operation{
	Method:        http.MethodPost,
	Path:          "/api/suggest",
	DefaultStatus: http.StatusOK,
	OperationID:   "suggest-from-image",
}

func SuggestFromImage(ctx context.Context, input *struct {
	RawBody huma.MultipartFormFiles[struct {
		File huma.FormFile `form:"file" required:"true"`
	}]
}) (output *struct{ Body ItemSuggestions }, err error) {
	_, user, _, _ := UserFromContext(ctx)
	addr, model, numCtx := effectiveOllamaConfig(user)
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

	suggestions, suggestErr := generateItemSuggestions(addr, model, numCtx, data)
	if suggestErr != nil {
		Log.Errorw("ollama suggest failed", "error", suggestErr)
		err = huma.Error503ServiceUnavailable("suggestion failed: " + suggestErr.Error())
		return
	}

	output = &struct{ Body ItemSuggestions }{Body: suggestions}
	return
}
