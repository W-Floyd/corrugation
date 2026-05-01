package backend

// GlobalConfig is a singleton table (always ID=1) storing server-wide settings.
type GlobalConfig struct {
	Model
	LogLevel                          string
	BackfillRecordEmbeddingsOnStart   bool
	BackfillArtifactEmbeddingsOnStart bool
	BackfillArtifactOwnersOnStart     bool
	AllowLocalUsernameLogin           bool
	InfinityTextModel                 string
	InfinityImageModel                string
	InfinityTextQueryPrefix           string
	InfinityTextDocumentPrefix        string
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

func SetInitialAllowLocalUsernameLogin(enabled bool) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.AllowLocalUsernameLogin = enabled
	return saveGlobalConfig(cfg)
}

type BackfillFlags struct {
	RecordEmbeddings   bool
	ArtifactEmbeddings bool
	ArtifactOwners     bool
}

func BackfillOnStartFlags() BackfillFlags {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return BackfillFlags{}
	}
	return BackfillFlags{
		RecordEmbeddings:   cfg.BackfillRecordEmbeddingsOnStart,
		ArtifactEmbeddings: cfg.BackfillArtifactEmbeddingsOnStart,
		ArtifactOwners:     cfg.BackfillArtifactOwnersOnStart,
	}
}
