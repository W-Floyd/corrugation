package backend

import (
	"sync"
)

// User stores user identity and per-user runtime-overridable settings. Username is empty when auth is disabled.
type User struct {
	Model
	Username                   string `gorm:"uniqueIndex"`
	IsAdmin                    bool
	InfinityTextModel          *string
	InfinityImageModel         *string
	InfinityTextQueryPrefix    *string
	InfinityTextDocumentPrefix *string
}

var userCache sync.Map // username → User

func userUsername(u *User) *string {
	if u == nil {
		return nil
	}
	return &u.Username
}

func loadUser(username string) (User, error) {
	if v, ok := userCache.Load(username); ok {
		return v.(User), nil
	}
	var u User
	err := db.Where(User{Username: username}).FirstOrCreate(&u).Error
	if err == nil {
		userCache.Store(username, u)
	}
	return u, err
}

func invalidateUserCache(username string) {
	userCache.Delete(username)
}

func countAdmins() (int64, error) {
	var count int64
	err := db.Model(&User{}).Where("is_admin = ?", true).Count(&count).Error
	return count, err
}

// EnsureFirstAdmin makes the given user an admin if no admins exist yet.
// Returns true if the user was just promoted.
func EnsureFirstAdmin(username string) (bool, error) {
	if username == "" {
		return false, nil
	}
	count, err := countAdmins()
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	u, err := loadUser(username)
	if err != nil {
		return false, err
	}
	u.IsAdmin = true
	if err = db.Save(&u).Error; err != nil {
		return false, err
	}
	invalidateUserCache(username)
	return true, nil
}

func saveUser(u User) error {
	err := db.Where(User{Username: u.Username}).Assign(u).FirstOrCreate(&u).Error
	if err == nil {
		userCache.Store(u.Username, u)
	}
	return err
}

// effectiveInfinityConfig returns the infinity config for a user.
// Priority: per-user override → GlobalConfig → CLI package vars.
func effectiveInfinityConfig(u *User) (text, image, queryPrefix, docPrefix string) {
	text = infinityTextModel
	image = infinityImageModel
	queryPrefix = infinityTextQueryPrefix
	docPrefix = infinityTextDocumentPrefix

	if cfg, err := loadGlobalConfig(); err == nil {
		if cfg.InfinityTextModel != "" {
			text = cfg.InfinityTextModel
		}
		if cfg.InfinityImageModel != "" {
			image = cfg.InfinityImageModel
		}
		if cfg.InfinityTextQueryPrefix != "" {
			queryPrefix = cfg.InfinityTextQueryPrefix
		}
		docPrefix = cfg.InfinityTextDocumentPrefix
	}

	if u != nil {
		if u.InfinityTextModel != nil {
			text = *u.InfinityTextModel
		}
		if u.InfinityImageModel != nil {
			image = *u.InfinityImageModel
		}
		if u.InfinityTextQueryPrefix != nil {
			queryPrefix = *u.InfinityTextQueryPrefix
		}
		if u.InfinityTextDocumentPrefix != nil {
			docPrefix = *u.InfinityTextDocumentPrefix
		}
	}
	return
}
