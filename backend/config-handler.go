package backend

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type GlobalConfigBody struct {
	LogLevel                  string `json:"logLevel" doc:"Log level: silent, panic, error, warn, info, debug"`
	GenerateEmbeddingsOnStart bool   `json:"generateEmbeddingsOnStart" doc:"Run embedding backfill on server startup"`
	AllowLocalUsernameLogin   bool   `json:"allowLocalUsernameLogin" doc:"Allow local username login without authentication for testing"`
}

type UserConfigBody struct {
	InfinityTextModel          *string `json:"infinityTextModel,omitempty" doc:"Override Infinity text embeddings model ID"`
	InfinityImageModel         *string `json:"infinityImageModel,omitempty" doc:"Override Infinity image embeddings model ID"`
	InfinityTextQueryPrefix    *string `json:"infinityTextQueryPrefix,omitempty" doc:"Override prefix prepended to text search queries"`
	InfinityTextDocumentPrefix *string `json:"infinityTextDocumentPrefix,omitempty" doc:"Override prefix prepended to text documents"`
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
		LogLevel:                  cfg.LogLevel,
		GenerateEmbeddingsOnStart: cfg.GenerateEmbeddingsOnStart,
		AllowLocalUsernameLogin:   cfg.AllowLocalUsernameLogin,
	}}
	return
}

var PutGlobalConfigOperation = huma.Operation{
	Method:        http.MethodPut,
	Path:          "/api/config/global",
	DefaultStatus: http.StatusOK,
}

func PutGlobalConfig(_ context.Context, input *struct {
	Body GlobalConfigBody
}) (output *struct{ Body GlobalConfigBody }, err error) {
	cfg := GlobalConfig{
		LogLevel:                  input.Body.LogLevel,
		GenerateEmbeddingsOnStart: input.Body.GenerateEmbeddingsOnStart,
		AllowLocalUsernameLogin:   input.Body.AllowLocalUsernameLogin,
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
	output = &struct{ Body UserConfigBody }{Body: UserConfigBody{
		InfinityTextModel:          u.InfinityTextModel,
		InfinityImageModel:         u.InfinityImageModel,
		InfinityTextQueryPrefix:    u.InfinityTextQueryPrefix,
		InfinityTextDocumentPrefix: u.InfinityTextDocumentPrefix,
	}}
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
	uc := User{
		Username:                   UsernameFromContext(ctx),
		InfinityTextModel:          input.Body.InfinityTextModel,
		InfinityImageModel:         input.Body.InfinityImageModel,
		InfinityTextQueryPrefix:    input.Body.InfinityTextQueryPrefix,
		InfinityTextDocumentPrefix: input.Body.InfinityTextDocumentPrefix,
	}
	if err = saveUser(uc); err != nil {
		return
	}
	output = &struct{ Body UserConfigBody }{Body: input.Body}
	return
}
