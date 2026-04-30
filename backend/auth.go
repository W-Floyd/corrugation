package backend

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/labstack/gommon/log"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type OIDCConfig struct {
	Issuer                string `json:"issuer"`
	JWKSURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

func FetchOIDCConfig(discoveryURL string, insecureSkipVerify bool) (*OIDCConfig, error) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
		},
	}
	resp, err := client.Get(discoveryURL) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cfg OIDCConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// AuthFrontendConfig is returned to the frontend so it can initiate the OIDC flow.
type AuthFrontendConfig struct {
	Enabled               bool   `json:"enabled"`
	AuthorizationEndpoint string `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint         string `json:"tokenEndpoint,omitempty"`
	ClientID              string `json:"clientId,omitempty"`
}

var globalAuthConfig = AuthFrontendConfig{}

func SetAuthConfig(cfg AuthFrontendConfig) {
	globalAuthConfig = cfg
}

var GetAuthConfigOperation = huma.Operation{
	Method:      http.MethodGet,
	Path:        "/api/auth/config",
	OperationID: "get-auth-config",
	Summary:     "Get OIDC auth configuration for the frontend",
}

var LoginLocalOperation = huma.Operation{
	Method:      http.MethodPost,
	Path:        "/api/auth/local/login",
	OperationID: "local-login",
	Summary:     "Login with username only (for local testing)",
}

func GetAuthConfigHandler(_ context.Context, _ *struct{}) (*struct{ Body AuthFrontendConfig }, error) {
	return &struct{ Body AuthFrontendConfig }{Body: globalAuthConfig}, nil
}

func LoginLocalHandler(_ context.Context, input *struct {
	Body struct {
		Username string `json:"username" required:"true" doc:"Username for local login"`
	}
}) (*struct {
	Body struct {
		Username string `json:"username"`
	}
}, error) {
	cfg, err := loadGlobalConfig()
	if err != nil {
		return nil, err
	}
	if !cfg.AllowLocalUsernameLogin {
		return nil, huma.Error403Forbidden("local username login is disabled")
	}
	return &struct {
		Body struct {
			Username string `json:"username"`
		}
	}{Body: struct {
		Username string `json:"username"`
	}{Username: input.Body.Username}}, nil
}

func RegisterAuthHandlers(api huma.API) {
	huma.Register(api, GetAuthConfigOperation, GetAuthConfigHandler)
	huma.Register(api, LoginLocalOperation, LoginLocalHandler)
}

type contextKey string

const usernameContextKey contextKey = "username"

func UsernameFromContext(ctx context.Context) (username string) {
	username, _ = ctx.Value(usernameContextKey).(string)
	return
}

func UserFromContext(ctx context.Context) (username string, user *User, userID *uint, err error) {
	username = UsernameFromContext(ctx)
	if username != "" {
		var u User
		u, err = loadUser(username)
		if err != nil {
			return
		}
		user = &u
		userID = &user.ID
	}
	return
}

func newJWKSet(jwksURL string, insecureSkipVerify bool) jwk.Set {
	registerOpts := []jwk.RegisterOption{jwk.WithMinRefreshInterval(10 * time.Minute)}
	if insecureSkipVerify {
		registerOpts = append(registerOpts, jwk.WithHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		}))
	}
	cache := jwk.NewCache(context.Background())
	if err := cache.Register(jwksURL, registerOpts...); err != nil {
		panic("failed to register jwk location: " + err.Error())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		panic("failed to fetch jwks on startup: " + err.Error())
	}
	return jwk.NewCachedSet(cache, jwksURL)
}

type cachedToken struct {
	username  string
	expiresAt time.Time
}

// ValidateToken resolves a raw JWT string to a username. Set by NewAuthMiddleware;
// nil when auth is disabled (all connections are anonymous).
var ValidateToken func(token string) (username string, err error)

// NewAuthMiddleware guards all /api/ paths (except /api/auth/) with OIDC JWT validation.
func NewAuthMiddleware(api huma.API, issuer, jwksURL string, insecureSkipVerify bool) func(huma.Context, func(huma.Context)) {
	keySet := newJWKSet(jwksURL, insecureSkipVerify)
	var tokenCache sync.Map // token string → cachedToken

	resolve := func(token string) (string, error) {
		if v, ok := tokenCache.Load(token); ok {
			cached := v.(cachedToken)
			if time.Now().Before(cached.expiresAt) {
				return cached.username, nil
			}
			tokenCache.Delete(token)
		}
		parsed, err := jwt.ParseString(token,
			jwt.WithKeySet(keySet),
			jwt.WithValidate(true),
			jwt.WithIssuer(issuer),
		)
		if err != nil {
			return "", err
		}
		username, _ := parsed.Get("preferred_username")
		usernameStr, _ := username.(string)
		tokenCache.Store(token, cachedToken{username: usernameStr, expiresAt: parsed.Expiration()})
		return usernameStr, nil
	}

	ValidateToken = resolve

	return func(ctx huma.Context, next func(huma.Context)) {
		path := ctx.URL().Path
		if !strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/auth/") {
			next(ctx)
			return
		}
		token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
		if token == "" {
			header := http.Header{}
			header.Add("Cookie", ctx.Header("Cookie"))
			req := http.Request{Header: header}
			if c, cookieErr := req.Cookie("auth_token"); cookieErr == nil {
				token = c.Value
			}
		}
		if token == "" {
			log.Warnf("[auth] missing token: %s %s", ctx.Method(), ctx.URL().Path)
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}
		// When local username login is enabled, treat the Bearer token as a plain username.
		cfg, err := loadGlobalConfig()
		if err == nil && cfg.AllowLocalUsernameLogin {
			ctx = huma.WithValue(ctx, usernameContextKey, token)
			next(ctx)
			return
		}
		usernameStr, err := resolve(token)
		if err != nil {
			log.Warnf("[auth] invalid token: %s %s: %v", ctx.Method(), ctx.URL().Path, err)
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "Unauthorized")
			return
		}
		ctx = huma.WithValue(ctx, usernameContextKey, usernameStr)
		next(ctx)
	}
}
