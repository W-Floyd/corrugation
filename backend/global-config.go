package backend

// GlobalConfig is a singleton table (always ID=1) storing server-wide settings.
type GlobalConfig struct {
	Model
	LogLevel                          string
	BackfillLegacyEmbeddingsOnStart   bool
	BackfillRecordEmbeddingsOnStart   bool
	BackfillArtifactEmbeddingsOnStart bool
	BackfillArtifactOwnersOnStart     bool
	AllowLocalUsernameLogin           bool
	InfinityTextModel                 string
	InfinityImageModel                string
	InfinityTextQueryPrefix           string
	InfinityTextDocumentPrefix        string
	// Comma-separated enabled barcode formats (e.g. "QR_CODE,CODE_128").
	// Empty string disables barcode detection entirely.
	EnabledBarcodeFormats string
	// nil = use full model output; positive value caps embedding dimensions via Infinity.
	MaximumEmbeddingDimensions *uint
	OllamaAddress              string
	OllamaVisionModel          string
	OllamaNumCtx               int
	OllamaImageMaxDim          int
	OllamaSuggestPrompt        string
	BackfillSuggestionsOnStart bool
}

func loadGlobalConfig() (GlobalConfig, error) {
	var cfg GlobalConfig
	err := db.FirstOrCreate(&cfg, GlobalConfig{Model: Model{ID: 1}, AllowLocalUsernameLogin: false}).Error
	return cfg, err
}

func saveGlobalConfig(cfg GlobalConfig) error {
	cfg.ID = 1
	return db.Save(&cfg).Error
}

// SetInitialInfinityConfig seeds GlobalConfig with CLI flag values only when
// the fields are empty (i.e. first run or never set via the settings page).
func SetInitialInfinityConfig(text, image, queryPrefix, docPrefix string) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	changed := false
	if cfg.InfinityTextModel == "" {
		cfg.InfinityTextModel = text
		changed = true
	}
	if cfg.InfinityImageModel == "" {
		cfg.InfinityImageModel = image
		changed = true
	}
	if cfg.InfinityTextQueryPrefix == "" {
		cfg.InfinityTextQueryPrefix = queryPrefix
		changed = true
	}
	// docPrefix is legitimately empty, so seed unconditionally on first run
	// (detected by text model being empty before the update above).
	if changed {
		cfg.InfinityTextDocumentPrefix = docPrefix
	}
	if changed {
		return saveGlobalConfig(cfg)
	}
	return nil
}

// SetInitialOllamaConfig writes CLI/env-var values to GlobalConfig.
// Non-empty/non-zero values always overwrite the DB so the compose file wins on
// every restart. Empty/zero values only seed when the DB field is unset.
// The prompt is special: empty means "use built-in constant" and only seeds.
func SetInitialOllamaConfig(address, visionModel string, numCtx, imageMaxDim int, suggestPrompt string) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	changed := false
	set := func(dst *string, val string) {
		if val != "" && *dst != val {
			*dst = val
			changed = true
		}
	}
	setInt := func(dst *int, val int) {
		if val > 0 && *dst != val {
			*dst = val
			changed = true
		}
	}
	set(&cfg.OllamaAddress, address)
	set(&cfg.OllamaVisionModel, visionModel)
	setInt(&cfg.OllamaNumCtx, numCtx)
	setInt(&cfg.OllamaImageMaxDim, imageMaxDim)
	if cfg.OllamaSuggestPrompt == "" {
		p := suggestPrompt
		if p == "" {
			p = ollamaSuggestPrompt
		}
		cfg.OllamaSuggestPrompt = p
		changed = true
	}
	if changed {
		return saveGlobalConfig(cfg)
	}
	return nil
}

func SetInitialAllowLocalUsernameLogin(enabled bool) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.AllowLocalUsernameLogin = enabled
	return saveGlobalConfig(cfg)
}

type BackfillFlags struct {
	LegacyEmbeddings   bool
	RecordEmbeddings   bool
	ArtifactEmbeddings bool
	ArtifactOwners     bool
	Suggestions        bool
}

func BackfillOnStartFlags() BackfillFlags {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return BackfillFlags{}
	}
	return BackfillFlags{
		LegacyEmbeddings:   cfg.BackfillLegacyEmbeddingsOnStart,
		RecordEmbeddings:   cfg.BackfillRecordEmbeddingsOnStart,
		ArtifactEmbeddings: cfg.BackfillArtifactEmbeddingsOnStart,
		ArtifactOwners:     cfg.BackfillArtifactOwnersOnStart,
		Suggestions:        cfg.BackfillSuggestionsOnStart,
	}
}
