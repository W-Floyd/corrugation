package backend

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// requireAdmin returns an error if auth is enabled and the current user is not an admin.
func requireAdmin(ctx context.Context) error {
	if ValidateToken == nil {
		return nil // auth disabled — no user concept, allow all
	}
	username := UsernameFromContext(ctx)
	if username == "" {
		return huma.Error403Forbidden("admin required")
	}
	u, err := loadUser(username)
	if err != nil {
		return err
	}
	if !u.IsAdmin {
		return huma.Error403Forbidden("admin required")
	}
	return nil
}

type GlobalConfigBody struct {
	LogLevel                          string   `json:"logLevel" doc:"Log level: silent, panic, error, warn, info, debug"`
	BackfillLegacyEmbeddingsOnStart   bool     `json:"backfillLegacyEmbeddingsOnStart" doc:"Delete legacy JSON-format embeddings on startup so they are regenerated"`
	BackfillRecordEmbeddingsOnStart   bool     `json:"backfillRecordEmbeddingsOnStart" doc:"Backfill missing record text embeddings on server startup"`
	BackfillArtifactEmbeddingsOnStart bool     `json:"backfillArtifactEmbeddingsOnStart" doc:"Backfill missing artifact image embeddings on server startup"`
	BackfillArtifactOwnersOnStart     bool     `json:"backfillArtifactOwnersOnStart" doc:"Assign owners to ownerless artifacts on server startup"`
	AllowLocalUsernameLogin           bool     `json:"allowLocalUsernameLogin" doc:"Allow local username login without authentication for testing"`
	InfinityTextModel                 string   `json:"infinityTextModel" doc:"Server-wide default text embedding model"`
	InfinityImageModel                string   `json:"infinityImageModel" doc:"Server-wide default image embedding model"`
	InfinityTextQueryPrefix           string   `json:"infinityTextQueryPrefix" doc:"Server-wide default text query prefix"`
	InfinityTextDocumentPrefix        string   `json:"infinityTextDocumentPrefix" doc:"Server-wide default text document prefix"`
	EnabledBarcodeFormats             []string `json:"enabledBarcodeFormats" doc:"Barcode/QR formats to detect on image upload. Valid: AZTEC, CODABAR, CODE_39, CODE_128, DATA_MATRIX, EAN_8, EAN_13, ITF, PDF_417, QR_CODE, RSS_14, RSS_EXPANDED, UPC_A, UPC_E. Empty disables scanning."`
	// nil = use full model output; positive value caps embedding dimensions via Infinity.
	MaximumEmbeddingDimensions *uint  `json:"maximumEmbeddingDimensions,omitempty" doc:"Cap embedding dimensions sent to Infinity. nil = use model default."`
	OllamaAddress              string `json:"ollamaAddress" doc:"Ollama service address for image content suggestions"`
	OllamaVisionModel          string `json:"ollamaVisionModel" doc:"Ollama vision model for image content suggestions"`
}

type UserConfigBody struct {
	InfinityTextModel          *string   `json:"infinityTextModel,omitempty" doc:"Override Infinity text embeddings model ID"`
	InfinityImageModel         *string   `json:"infinityImageModel,omitempty" doc:"Override Infinity image embeddings model ID"`
	InfinityTextQueryPrefix    *string   `json:"infinityTextQueryPrefix,omitempty" doc:"Override prefix prepended to text search queries"`
	InfinityTextDocumentPrefix *string   `json:"infinityTextDocumentPrefix,omitempty" doc:"Override prefix prepended to text documents"`
	EnabledBarcodeFormats      *[]string `json:"enabledBarcodeFormats,omitempty" doc:"Override barcode/QR formats to detect. null = use global default; [] = disable; non-empty = specific formats."`
	// nil = inherit global/model default; positive value overrides embedding dimensions for this user.
	MaximumEmbeddingDimensions *uint `json:"maximumEmbeddingDimensions,omitempty" doc:"Override embedding dimensions. null = inherit global; positive = cap to this value."`
}

func barcodeFormatsToSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func barcodeFormatsToString(formats []string) string {
	return strings.Join(formats, ",")
}

// SetInitialLogLevel is called at startup with the flag value. Always persists to DB.
func SetInitialLogLevel(level string) error {
	SetLogLevel(level)
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.LogLevel = level
	return saveGlobalConfig(cfg)
}

// --- Global config ---

var GetGlobalConfigOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/config/global",
	DefaultStatus: http.StatusOK,
}

func GetGlobalConfig(_ context.Context, _ *struct{}) (output *struct{ Body GlobalConfigBody }, err error) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return
	}
	output = &struct{ Body GlobalConfigBody }{Body: GlobalConfigBody{
		LogLevel:                          cfg.LogLevel,
		BackfillLegacyEmbeddingsOnStart:   cfg.BackfillLegacyEmbeddingsOnStart,
		BackfillRecordEmbeddingsOnStart:   cfg.BackfillRecordEmbeddingsOnStart,
		BackfillArtifactEmbeddingsOnStart: cfg.BackfillArtifactEmbeddingsOnStart,
		BackfillArtifactOwnersOnStart:     cfg.BackfillArtifactOwnersOnStart,
		AllowLocalUsernameLogin:           cfg.AllowLocalUsernameLogin,
		InfinityTextModel:                 cfg.InfinityTextModel,
		InfinityImageModel:                cfg.InfinityImageModel,
		InfinityTextQueryPrefix:           cfg.InfinityTextQueryPrefix,
		InfinityTextDocumentPrefix:        cfg.InfinityTextDocumentPrefix,
		EnabledBarcodeFormats:             barcodeFormatsToSlice(cfg.EnabledBarcodeFormats),
		MaximumEmbeddingDimensions:        cfg.MaximumEmbeddingDimensions,
		OllamaAddress:                     cfg.OllamaAddress,
		OllamaVisionModel:                 cfg.OllamaVisionModel,
	}}
	return
}

var PutGlobalConfigOperation = huma.Operation{
	Method:        http.MethodPut,
	Path:          "/api/config/global",
	DefaultStatus: http.StatusOK,
}

func PutGlobalConfig(ctx context.Context, input *struct {
	Body GlobalConfigBody
}) (output *struct{ Body GlobalConfigBody }, err error) {
	if err = requireAdmin(ctx); err != nil {
		return
	}
	cfg := GlobalConfig{
		LogLevel:                          input.Body.LogLevel,
		BackfillLegacyEmbeddingsOnStart:   input.Body.BackfillLegacyEmbeddingsOnStart,
		BackfillRecordEmbeddingsOnStart:   input.Body.BackfillRecordEmbeddingsOnStart,
		BackfillArtifactEmbeddingsOnStart: input.Body.BackfillArtifactEmbeddingsOnStart,
		BackfillArtifactOwnersOnStart:     input.Body.BackfillArtifactOwnersOnStart,
		AllowLocalUsernameLogin:           input.Body.AllowLocalUsernameLogin,
		InfinityTextModel:                 input.Body.InfinityTextModel,
		InfinityImageModel:                input.Body.InfinityImageModel,
		InfinityTextQueryPrefix:           input.Body.InfinityTextQueryPrefix,
		InfinityTextDocumentPrefix:        input.Body.InfinityTextDocumentPrefix,
		EnabledBarcodeFormats:             barcodeFormatsToString(input.Body.EnabledBarcodeFormats),
		MaximumEmbeddingDimensions:        input.Body.MaximumEmbeddingDimensions,
		OllamaAddress:                     input.Body.OllamaAddress,
		OllamaVisionModel:                 input.Body.OllamaVisionModel,
	}
	if err = saveGlobalConfig(cfg); err != nil {
		return
	}
	SetLogLevel(cfg.LogLevel)
	output = &struct{ Body GlobalConfigBody }{Body: input.Body}
	return
}

// --- User config ---

var GetUserConfigOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/config/user",
	DefaultStatus: http.StatusOK,
}

func GetUserConfig(ctx context.Context, _ *struct{}) (output *struct{ Body UserConfigBody }, err error) {
	u, err := loadUser(UsernameFromContext(ctx))
	if err != nil {
		return
	}
	body := UserConfigBody{
		InfinityTextModel:          u.InfinityTextModel,
		InfinityImageModel:         u.InfinityImageModel,
		InfinityTextQueryPrefix:    u.InfinityTextQueryPrefix,
		InfinityTextDocumentPrefix: u.InfinityTextDocumentPrefix,
		MaximumEmbeddingDimensions: u.MaximumEmbeddingDimensions,
	}
	if u.EnabledBarcodeFormats != nil {
		s := barcodeFormatsToSlice(*u.EnabledBarcodeFormats)
		body.EnabledBarcodeFormats = &s
	}
	output = &struct{ Body UserConfigBody }{Body: body}
	return
}

var PutUserConfigOperation = huma.Operation{
	Method:        http.MethodPut,
	Path:          "/api/config/user",
	DefaultStatus: http.StatusOK,
}

func PutUserConfig(ctx context.Context, input *struct {
	Body UserConfigBody
}) (output *struct{ Body UserConfigBody }, err error) {
	username := UsernameFromContext(ctx)
	u, err := loadUser(username)
	if err != nil {
		return
	}
	u.InfinityTextModel = input.Body.InfinityTextModel
	u.InfinityImageModel = input.Body.InfinityImageModel
	u.InfinityTextQueryPrefix = input.Body.InfinityTextQueryPrefix
	u.InfinityTextDocumentPrefix = input.Body.InfinityTextDocumentPrefix
	u.MaximumEmbeddingDimensions = input.Body.MaximumEmbeddingDimensions
	if input.Body.EnabledBarcodeFormats != nil {
		s := barcodeFormatsToString(*input.Body.EnabledBarcodeFormats)
		u.EnabledBarcodeFormats = &s
	} else {
		u.EnabledBarcodeFormats = nil
	}
	if err = db.Save(&u).Error; err != nil {
		return
	}
	invalidateUserCache(username)
	output = &struct{ Body UserConfigBody }{Body: input.Body}
	return
}

// --- User management (admin only) ---

type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

var ListUsersOperation = huma.Operation{
	Method:        http.MethodGet,
	Path:          "/api/users",
	OperationID:   "list-users",
	DefaultStatus: http.StatusOK,
}

func ListUsers(ctx context.Context, _ *struct{}) (output *struct{ Body []UserInfo }, err error) {
	if err = requireAdmin(ctx); err != nil {
		return
	}
	var users []User
	if err = db.Find(&users).Error; err != nil {
		return
	}
	infos := make([]UserInfo, len(users))
	for i, u := range users {
		infos[i] = UserInfo{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin}
	}
	output = &struct{ Body []UserInfo }{Body: infos}
	return
}

var SetUserAdminOperation = huma.Operation{
	Method:        http.MethodPut,
	Path:          "/api/users/{username}/admin",
	OperationID:   "set-user-admin",
	DefaultStatus: http.StatusOK,
}

func SetUserAdmin(ctx context.Context, input *struct {
	Username string `path:"username"`
	Body     struct {
		IsAdmin bool `json:"isAdmin"`
	}
}) (output *struct{ Body UserInfo }, err error) {
	if err = requireAdmin(ctx); err != nil {
		return
	}
	if !input.Body.IsAdmin && input.Username == UsernameFromContext(ctx) {
		err = huma.Error403Forbidden("cannot remove admin from yourself")
		return
	}
	var u User
	if err = db.Where(User{Username: input.Username}).First(&u).Error; err != nil {
		err = huma.Error404NotFound("user not found")
		return
	}
	u.IsAdmin = input.Body.IsAdmin
	if err = db.Save(&u).Error; err != nil {
		return
	}
	invalidateUserCache(input.Username)
	output = &struct{ Body UserInfo }{Body: UserInfo{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin}}
	return
}
