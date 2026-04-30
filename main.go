package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"github.com/W-Floyd/corrugation/backend"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/danielgtaylor/huma/v2/autopatch"
	"github.com/danielgtaylor/huma/v2/humacli"
)

type Options struct {
	Address                    string `help:"Address to listen on" default:"0.0.0.0"`
	Port                       int    `help:"Port to listen on" default:"8083"`
	Dist                       string `help:"Dist path" default:"./dist"`
	Data                       string `help:"Data path" default:"./data"`
	OIDCDiscoveryURL           string `help:"OIDC discovery URL (e.g. https://authentik.example.com/application/o/<slug>/.well-known/openid-configuration); omit to disable auth"`
	OIDCClientID               string `help:"OAuth2 client ID registered in Authentik"`
	OIDCInsecureSkipVerify     bool   `help:"Skip TLS certificate verification for OIDC discovery and JWKS requests"`
	LogLevel                   string `help:"Log level: silent, error, warn, info" default:"warn"`
	BackfillOnStart            bool   `help:"Run backfill on server startup" default:"false"`
	AllowLocalUsernameLogin    bool   `help:"Allow local username login without OIDC for testing" default:"false"`
	EmbeddingConcurrency       int    `help:"Max parallel embedding requests" default:"4"`
	InfinityAddress            string `help:"Infinity embeddings server address" default:"http://localhost:8002"`
	InfinityTextModel          string `help:"Infinity text embeddings model ID" default:"BAAI/bge-large-en-v1.5"`
	InfinityImageModel         string `help:"Infinity image embeddings model ID" default:"openai/clip-vit-large-patch14"`
	InfinityTextQueryPrefix    string `help:"Prefix prepended to text search queries before embedding" default:"Represent this sentence for searching relevant passages: "`
	InfinityTextDocumentPrefix string `help:"Prefix prepended to text documents before embedding" default:""`
	LegacyImportUser           string `help:"Username for legacy imports" default:"legacy"`
	PprofAddr                  string `help:"pprof HTTP listener address; empty to disable" default:""`
}

func main() {

	cli := humacli.New(func(hooks humacli.Hooks, options *Options) {

		backend.Log.Info("init backend")
		backend.SetInfinityConfig(options.InfinityAddress, options.InfinityTextModel, options.InfinityImageModel, options.InfinityTextQueryPrefix, options.InfinityTextDocumentPrefix)
		backend.SetEmbeddingConcurrency(options.EmbeddingConcurrency)
		if options.PprofAddr != "" {
			go func() {
				backend.Log.Infof("pprof listening on %s", options.PprofAddr)
				if err := http.ListenAndServe(options.PprofAddr, nil); err != nil {
					backend.Log.Errorw("pprof server error", "error", err)
				}
			}()
		}

		if _, err := os.Stat(options.Data); os.IsNotExist(err) {
			err := os.Mkdir(options.Data, 0755)
			if err != nil {
				backend.Log.Fatal(err)
			}
		}

		dbPath := filepath.Join(options.Data, "db.sqlite")
		dbExists := fileExists(dbPath)

		err := backend.ConnectDB(dbPath)
		if err != nil {
			backend.Log.Fatal(err)
		}
		if err = backend.InitAndMigrateDB(); err != nil {
			backend.Log.Fatal(err)
		}
		if err = backend.SetInitialLogLevel(options.LogLevel); err != nil {
			backend.Log.Fatalf("failed to persist log level: %v", err)
		}
		if err = backend.SetInitialBackfillOnStart(options.BackfillOnStart); err != nil {
			backend.Log.Fatalf("failed to persist backfill-on-start: %v", err)
		}
		if err = backend.SetInitialAllowLocalUsernameLogin(options.AllowLocalUsernameLogin); err != nil {
			backend.Log.Fatalf("failed to persist allow-local-username-login: %v", err)
		}

		if !dbExists {
			if err := runLegacyMigration(options.Data, options.LegacyImportUser); err != nil {
				backend.Log.Infof("legacy migration failed: %v", err)
			}
		}

		// Create a new router & API
		router := http.NewServeMux()
		config := huma.DefaultConfig("My API", "1.0.0")

		var oidcCfg *backend.OIDCConfig
		if options.OIDCDiscoveryURL != "" {
			var err error
			oidcCfg, err = backend.FetchOIDCConfig(options.OIDCDiscoveryURL, options.OIDCInsecureSkipVerify)
			if err != nil {
				backend.Log.Fatalf("fetch OIDC config: %v", err)
			}
			config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
				"authentik": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
				},
			}
			backend.SetAuthConfig(backend.AuthFrontendConfig{
				Enabled:               true,
				AuthorizationEndpoint: oidcCfg.AuthorizationEndpoint,
				TokenEndpoint:         oidcCfg.TokenEndpoint,
				ClientID:              options.OIDCClientID,
			})
			backend.Log.Infof("OIDC auth enabled, issuer: %s", oidcCfg.Issuer)
		}

		api := humago.New(router, config)
		backend.RegisterAuthHandlers(api)

		if oidcCfg != nil {
			api.UseMiddleware(backend.NewAuthMiddleware(api, oidcCfg.Issuer, oidcCfg.JWKSURI, options.OIDCInsecureSkipVerify))
		}

		autopatch.AutoPatch(api)

		// Catch-all: serve from dist on each request; fallback to index.html for SPA.
		// Unknown /api/* paths get a 404 instead of the SPA HTML.
		if _, err := os.Stat(options.Dist); err == nil {
			dist := options.Dist
			router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					http.Error(w, "404 not found", http.StatusNotFound)
					return
				}
				path := filepath.Join(dist, filepath.Clean(r.URL.Path))
				if info, err := os.Stat(path); err == nil && !info.IsDir() {
					http.ServeFile(w, r, path)
					return
				}
				http.ServeFile(w, r, filepath.Join(dist, "index.html"))
			})
		}

		backend.RegisterHandlers(api)
		router.HandleFunc("GET /ws", backend.WsHandler)

		// Tell the CLI how to start your router.
		hooks.OnStart(func() {
			backend.StartEmbeddingWorkers()
			if backend.ShouldBackfillOnStart() {
				go backend.BackfillEmbeddings()
			}
			err := http.ListenAndServe(fmt.Sprintf("%s:%d", options.Address, options.Port), router)
			if err != nil {
				backend.Log.Fatal(err)
			}
		})

	})

	// Run the CLI. When passed no commands, it starts the server.
	cli.Run()

}

func runLegacyMigration(dataPath string, legacyImportUser string) error {
	tarPath := filepath.Join(dataPath, "legacy.tar.gz")
	storeJSON := filepath.Join(dataPath, "store.json")

	switch {
	case fileExists(storeJSON):
		backend.Log.Info("legacy store.json found, running migration")
		if err := buildLegacyTarGz(dataPath, tarPath); err != nil {
			return fmt.Errorf("build tar.gz: %w", err)
		}
	case fileExists(tarPath):
		backend.Log.Info("legacy.tar.gz found, running migration")
	default:
		return nil
	}

	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer f.Close()

	result, err := backend.ImportFromReader(context.Background(), f, false, legacyImportUser)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	backend.Log.Infof("legacy migration complete: %d entities, %d artifacts imported", result.EntitiesImported, result.ArtifactsImported)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func buildLegacyTarGz(dataPath, tarPath string) error {
	out, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	addFile := func(name, arcName string) error {
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name: arcName,
			Mode: 0644,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	}

	if err := addFile(filepath.Join(dataPath, "store.json"), "store.json"); err != nil {
		return fmt.Errorf("store.json: %w", err)
	}

	artifactsDir := filepath.Join(dataPath, "artifacts")
	entries, err := os.ReadDir(artifactsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read artifacts dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".webp") {
			continue
		}
		src := filepath.Join(artifactsDir, e.Name())
		if err := addFile(src, "artifacts/"+e.Name()); err != nil {
			backend.Log.Infof("skipping artifact %s: %v", e.Name(), err)
		}
	}

	return nil
}

type CustomResponseWriter struct {
	body       []byte
	statusCode int
	header     http.Header
}

func NewCustomResponseWriter() *CustomResponseWriter {
	return &CustomResponseWriter{
		header: http.Header{},
	}
}

func (w *CustomResponseWriter) Header() http.Header {
	return w.header
}

func (w *CustomResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return 0, nil
}

func (w *CustomResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
