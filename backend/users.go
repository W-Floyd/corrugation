package backend

import (
	"sync"
)

// User stores user identity and per-user runtime-overridable settings. Username is empty when auth is disabled.
type User struct {
	Model
	Username                   string `gorm:"uniqueIndex"`
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

func saveUser(u User) error {
	err := db.Where(User{Username: u.Username}).Assign(u).FirstOrCreate(&u).Error
	if err == nil {
		userCache.Store(u.Username, u)
	}
	return err
}

// effectiveInfinityConfig returns the infinity config for a user, falling back to env defaults for nil fields.
func effectiveInfinityConfig(u *User) (text, image, queryPrefix, docPrefix string) {
	text = infinityTextModel
	image = infinityImageModel
	queryPrefix = infinityTextQueryPrefix
	docPrefix = infinityTextDocumentPrefix

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
