package backend

import "gorm.io/gorm"

// GlobalConfig is a singleton table (always ID=1) storing server-wide settings.
type GlobalConfig struct {
	gorm.Model
	LogLevel                  string
	GenerateEmbeddingsOnStart bool
	AllowLocalUsernameLogin   bool
}

func loadGlobalConfig() (GlobalConfig, error) {
	var cfg GlobalConfig
	err := db.FirstOrCreate(&cfg, GlobalConfig{Model: gorm.Model{ID: 1}, AllowLocalUsernameLogin: false}).Error
	return cfg, err
}

func saveGlobalConfig(cfg GlobalConfig) error {
	cfg.ID = 1
	return db.Save(&cfg).Error
}

// SetInitialGenerateEmbeddingsOnStart is called at startup with the flag value. Always persists to DB.
func SetInitialGenerateEmbeddingsOnStart(enabled bool) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.GenerateEmbeddingsOnStart = enabled
	return saveGlobalConfig(cfg)
}

func SetInitialAllowLocalUsernameLogin(enabled bool) error {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return err
	}
	cfg.AllowLocalUsernameLogin = enabled
	return saveGlobalConfig(cfg)
}

func ShouldGenerateEmbeddingsOnStart() bool {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return false
	}
	return cfg.GenerateEmbeddingsOnStart
}
