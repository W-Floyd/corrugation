# Corrugation Backend - Technical Documentation

This file provides guidance to developers working on the Corrugation backend.

## Overview

Corrugation backend is a Go-based REST API server using the Huma framework. It provides entity management with hierarchical organization, AI-powered embedding search via Infinity, and real-time WebSocket updates.

## Technology Stack

- **Framework**: Huma v2 (built on `net/http`)
- **Database**: SQLite with GORM ORM
- **Connection Pool**: 10 idle, 10 open connections (via `SetMaxIdleConns`, `SetMaxOpenConns`)
- **Journal Mode**: WAL (Write-Ahead Logging) for concurrent reads
- **Cache Size**: -64000 pages (~64MB)
- **Authentication**: OIDC/JWT via Authentik (optional)
- **Embeddings**: Infinity server (OpenAI CLIP, BGE models)
- **Concurrency**: Goroutine worker pools with semaphores
- **WebSocket**: Hub pattern with username-based routing
- **Compression**: Gzip for imports/exports

## Core Data Models

### Record
Defined in `record.go` as the `Record` struct with fields: Quantity, ReferenceNumber, Title, Description, Tags, Artifacts, ParentID, OwnerID. `SearchConfidenceImage` and `SearchConfidenceText` are computed at runtime (marked `gorm:"-"`, not persisted). ReferenceNumber is unique per owner via `idx_owner_ref` index. ParentID creates hierarchical tree structure.

### Tag
Defined in `tag.go` as the `Tag` struct with Title as primary key and no duplicate titles allowed. Join table: `record_tags(record_id, tag_title)`.

### Artifact
Defined in `artifact.go` as the `Artifact` struct with Data, OriginalFilename, ContentType, SmallPreviewID, SmallPreview, LargePreviewID, LargePreview, RecordID fields. Supports image previews (625000 max pixel count WebP, 1250000 max pixel count WebP). Implements `ArtifactInterface` for image handling. Preview sizes are maximum pixel counts, not dimensions.

### Embedding
Defined in `embedding.go` as the `Embedding` struct with RecordID, ArtifactID, EmbedModel, Data, Hash fields. Composite index on `(record_id, artifact_id, embed_model)`. Deduplication by `hash` field using SHA-256.

### EmbeddingJob
Defined in `embedding-queue.go` as the `EmbeddingJob` struct with JobType, TargetID, OwnerID, Username, Status, ErrorMsg, RetryCount, EmbedModel, Source fields. Index on `(job_type, target_id, embed_model)` for deduplication. Retry mechanism: up to 5 retries on failure. Status transitions: pending → processing → done/failed.

### User
Defined in `users.go` as the `User` struct with Username, InfinityTextModel, InfinityImageModel, InfinityTextQueryPrefix, InfinityTextDocumentPrefix fields. Username empty string means anonymous access. Per-user Infinity model overrides supported. Cached in `userCache sync.Map`.

### GlobalConfig (Singleton)
Defined in `global-config.go` as the `GlobalConfig` struct with LogLevel and GenerateEmbeddingsOnStart fields. Always ID=1 (first create or update). Stores global server settings.

## API Endpoints

### Backfill
- `GET /api/config/global` - Get global config with `generateEmbeddingsOnStart` flag
- Backfill runs on startup when `generateEmbeddingsOnStart` is true

### Records
- `GET /api/record/{id}` - Get single record by ID
- `GET /api/records` - List records
  - Query params: `id`, `global`, `childrenDepth`, `parentDepth`, `search`, `searchImage`, `searchTextEmbedded`, `searchTextSubstring`, `minImageScore`, `minTextScore`, `timestamps`
  - Returns 207 Multi-Status for partial search results
- `POST /api/record` - Create record (returns 409 on duplicate reference number)
- `POST /api/record/{id}` - Update record (full replace)
- `DELETE /api/record/{id}` - Delete record

### Artifacts
- `POST /api/artifact` - Create artifact (multipart form)
- `GET /api/artifact/{id}` - Get artifact (returns WebP preview, ETag support)

### Tags
- `GET /api/tags` - List all tags
- `GET /api/tag/{id}` - Get tag by title
- `POST /api/tag` - Create tag
- `DELETE /api/tag/{id}` - Delete tag by title

### Config
- `GET /api/config/global` - Get global config
- `PUT /api/config/global` - Update global config
- `GET /api/config/user` - Get user config
- `PUT /api/config/user` - Update user config

### Auth
- `GET /api/auth/config` - Get OIDC config (enabled, endpoints, client ID)

### Embeddings
- `GET /api/embeddings/progress` - Get job status counts (total, pending, processing, done, failed)
- `GET /api/embeddings/search-progress` - Get search embedding progress by scope
   - Query params: `id`, `global`, `childrenDepth`, `searchImage`, `searchTextEmbedded`
   - Returns record and artifact completion status per user
- `POST /api/embeddings/flush` - Delete stale embeddings

### Import
- `POST /api/import` - Import legacy tar.gz
   - Query params: `reset` (clear existing data first)
   - Supports `store.json` and `artifacts/*.webp` from previous Corrugation installs

### Visualization
- `GET /api/records/visualize` - Generate HTML entity graph
- `GET /api/tags/visualize` - Generate HTML tag graph

## Embedding System Architecture

### Worker Pool
- Default 4 workers (configurable via `--embedding-concurrency`)
- Semaphore-controlled via `embeddingSemaphore chan struct{}`
- Two job queues: `embeddingJobQueue` (regular) and `embeddingSearchJobQueue` (search)

### Embedding Flow
1. **Enqueue**: `EnqueueEmbeddingJob()` checks for pending/processing jobs via WHERE clause (not database constraint)
2. **Health Check**: Workers block until Infinity server `/health` returns 200 OK
3. **Claim**: Atomic update from pending → processing via `RowsAffected == 0` check
4. **Fast Dedup**: Skip if embedding exists for target+model (counts existing)
5. **Generate**: Call Infinity `/embeddings` endpoint with model, encoding_format, input, modality
6. **Save**: Store embedding with SHA-256 hash key for deduplication
7. **Broadcast**: WebSocket message `embedding_progress:{type}:{id}` to user

### Models
- **Infinity Text**: `BAAI/bge-large-en-v1.5` (configurable)
- **Infinity Image**: `openai/clip-vit-large-patch14` (configurable)
- Per-user overrides supported via `User` model

### Deduplication
- Database level: Regular indexes on `(job_type, target_id, embed_model)` fields for deduplication queries (no UNIQUE constraint)
- Cache: `embeddingsCache sync.Map` with SHA-256 hash keys for vectors
- Skip existing embeddings before enqueuing via `Count` query

### Retry Mechanism
- Failed jobs retry up to 5 times (`maxEmbeddingRetries`)
- Periodic scan (30s interval) rescues pending jobs from channel overflow
- `retryTrigger` channel coalesces rapid failures (10s interval for retry check)

## Database Schema

All database tables are managed via GORM models defined in the backend source files. Note that `SearchConfidenceImage` and `SearchConfidenceText` fields in the Record struct are marked `gorm:"-"` and are not persisted to the database.

- `records` → `record.go` Record struct
- `tags` → `tag.go` Tag struct  
- `record_tags` → Join table for `record.go` many-to-many with `tag.go`
- `artifacts` → `artifact.go` Artifact struct
- `embeddings` → `embedding.go` Embedding struct
- `embedding_jobs` → `embedding-queue.go` EmbeddingJob struct
- `users` → `users.go` User struct
- `global_config` → `global-config.go` GlobalConfig struct (singleton ID=1)

## Database Configuration

- **Connection Pool**: 10 idle, 10 open connections
- **Journal Mode**: WAL (Write-Ahead Logging) for concurrent reads
- **Cache Size**: -64000 pages (~64MB)
- **File Mode**: SQLite file only (no network)

## Authentication

### OIDC Flow
1. Frontend fetches `/api/auth/config` at startup
2. Frontend performs PKCE OAuth flow to Authentik
3. Backend validates JWT via JWKS (cached, 10min refresh)
4. Username stored in context via `UsernameFromContext()`
5. Auth disabled when `OIDCDiscoveryURL` flag omitted

### Middleware
- Guards `/api/*` paths (excludes `/api/auth/`)
- Extracts token from: query param, Authorization header, or `auth_token` cookie
- Returns 401 Unauthorized on invalid tokens

### Anonymous Mode
- Empty username allows all operations
- Context key `usernameContextKey` returns ""
- WebSocket connections treated as anonymous

### WebSocket Protocol

### Hub Pattern
Defined in `ws.go` as the `hub` struct with `mu` sync.Mutex and `clients map[*websocket.Conn]string`. Single `wsHub` instance manages all connections. Username-based routing for targeted broadcasts. **No auto-reconnect** - frontend must implement reconnection logic. Token extracted from `?token=` query param (WebSocket cannot set headers).

### Messages
- `update` - Reload entity list (all clients)
- `embedding_server_online` - Embedding server available
- `embedding_server_offline` - Embedding server unavailable
- `embedding_progress:{type}:{id}` - Job progress for specific item

### Connection Handling
- Token extracted from `?token=` query param (WebSocket cannot set headers)
- `ws.Upgrader` allows all origins (no origin validation)

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8083` | HTTP listen port |
| `--address` | `0.0.0.0` | HTTP listen address |
| `--dist` | `./dist` | Frontend static files path |
| `--data` | `./data` | Database directory |
| `--oidc-discovery-url` | N/A | OIDC discovery URL |
| `--oidc-client-id` | N/A | OAuth2 client ID |
| `--oidc-insecure-skip-verify` | `false` | Skip TLS verify for OIDC |
| `--log-level` | `warn` | Log level (silent, error, warn, info, debug) |
| `--generate-embeddings-on-start` | `false` | Run backfill on startup |
| `--embedding-concurrency` | `4` | Max parallel embedding requests |
| `--infinity-address` | `http://localhost:8002` | Infinity server URL |
| `--infinity-text-model` | `BAAI/bge-large-en-v1.5` | Text embedding model |
| `--infinity-image-model` | `openai/clip-vit-large-patch14` | Image embedding model |
| `--infinity-text-query-prefix` | `Represent this sentence for searching relevant passages: ` | Query prefix |
| `--infinity-text-document-prefix` | `` | Document prefix |
| `--legacy-import-user` | `legacy` | Username for legacy imports |
| `--pprof-addr` | `` | pprof HTTP listener |

## Key Functions

Refer to the actual source files for implementation details:
- `constants.go`: Error strings, Infinity defaults, SetInfinityConfig(), SetEmbeddingConcurrency()
- `db.go`: ConnectDB(), InitAndMigrateDB()
- `record.go`: Record model, GenerateEmbeddings(), GetRecordEmbeddings()
- `artifact.go`: Artifact model, ArtifactInterface, Image type, GenerateEmbeddings()
- `embedding.go`: Embedding model, saveEmbedding()
- `embedding-queue.go`: Worker pool, EnqueueEmbeddingJob(), StartEmbeddingWorkers()
- `embedding-handler.go`: GetEmbeddingProgress(), GetSearchEmbeddingProgress()
- `infinity.go`: Infinity client calls, embeddingsCache, GenerateTextDocumentEmbeddingsCtx(), GenerateTextQueryEmbeddingsCtx(), GenerateImageQueryEmbeddingsCtx()
- `search.go`: SearchByRecord(), SearchByArtifact()
- `import.go`: ImportFromReader()
- `ws.go`: Broadcast(), BroadcastAll(), BroadcastToUser(), WsHandler()
- `record-handler.go`: ListRecords(), GetRecord(), CreateRecord(), UpdateRecord(), PatchRecord(), DeleteRecord()
- `tag-handler.go`: ListTags(), GetTag(), CreateTag(), DeleteTag()
- `config-handler.go`: GetGlobalConfig(), PutGlobalConfig(), GetUserConfig(), PutUserConfig()
- `auth.go`: GetAuthConfigHandler(), NewAuthMiddleware(), FetchOIDCConfig()
- `backfill.go`: BackfillEmbeddings(), backfillRecordEmbeddings(), backfillArtifactEmbeddings()

### Database
- `ConnectDB(path)` - Open SQLite connection with WAL mode
- `InitAndMigrateDB()` - AutoMigrate all models on startup

### Embedding
- `StartEmbeddingWorkers()` - Launch worker goroutines (4 default, configurable)
- `BackfillEmbeddings()` - Full backfill for all records/artifacts by owner
- `EnqueueEmbeddingJob(jobType, targetID, ownerID, username, embedModel, source)` - Add to queue
- `GetRecordEmbeddings(ctx, scopedIDs)` - Fetch record embeddings for scope
- `GetArtifactEmbeddings(ctx, artifactRecordMap)` - Fetch artifact embeddings
- `GenerateTextDocumentEmbeddingsCtx`, `GenerateTextQueryEmbeddingsCtx`, `GenerateImageQueryEmbeddingsCtx` - Infinity client calls

### Search
- `SearchByRecord(ctx, query, scopedIDs)` - Text embedding search
- `SearchByArtifact(ctx, query, artifactRecordMap)` - Image embedding search
- `GetRecordEmbeddings(ctx, scopedIDs)` - Fetch record embeddings
- `GetArtifactEmbeddings(ctx, artifactRecordMap)` - Fetch artifact embeddings

### Import
- `ImportFromReader(ctx, r, reset, username)` - Import legacy tar.gz

### WebSocket
- `Broadcast()` - Signal all clients to reload
- `BroadcastAll(msg)` - Broadcast to all clients
- `BroadcastToUser(username, msg)` - Broadcast to specific user

## Entry Point (`main.go`)

1. Parse CLI flags via `humacli`
2. Set Infinity config and embedding concurrency globally
3. Create data directory if missing
4. Connect to SQLite database with WAL mode, 10 conn pool, -64000 cache size
5. Run auto-migrations for all models
6. Initialize auth config (OIDC discovery, JWKS cache with 10min refresh)
7. Register Huma handlers at `/api/*` paths
8. Register WebSocket handler at `/ws` (allows all origins)
9. Start embedding workers (waits for Infinity health check)
10. Trigger backfill if `generateEmbeddingsOnStart` flag set
11. Listen on `:8083` (configurable via `--port`, `--address`)

## File Structure

```
backend/
├── artifact.go           # Artifact model, ArtifactInterface, Image type
├── artifact-handler.go   # Artifact CRUD endpoints
├── auth.go               # OIDC config, AuthFrontendConfig, NewAuthMiddleware
├── backfill.go           # BackfillEmbeddings() logic
├── config-handler.go     # Global/user config endpoints
├── constants.go          # Error strings, Infinity defaults
├── db.go                 # ConnectDB(), InitAndMigrateDB()
├── embedding.go          # Embedding model, saveEmbedding()
├── embedding-handler.go  # Embedding progress endpoints
├── embedding-queue.go    # Worker pool, job queue, dedup
├── global-config.go      # GlobalConfig singleton
├── handlers.go           # Register all Huma endpoints
├── import.go             # Legacy tar.gz import
├── infinity.go           # Infinity client calls, embeddings cache
├── logger.go             # Structured logging
├── record.go             # Record model, embedding gen
├── record-handler.go     # Record CRUD, next reference
├── record-helper.go      # Search query builder, hierarchy
├── search.go              # Vector search implementation
├── tag.go                 # Tag model
├── tag-handler.go         # Tag CRUD endpoints
├── tag-helper.go          # Tag query helpers
├── users.go              # User model, cache, effective config
├── utils.go              # Helper functions
└── ws.go                 # WebSocket hub, broadcast logic
```

## Design Patterns

- **Repository Pattern**: GORM `gorm.G[T](db)` for all CRUD operations
- **Worker Pool**: Fixed goroutines with buffered channel queue (4096 capacity, 4 workers default)
- **Cache-Aside**: `embeddingsCache sync.Map` for vector caching with SHA-256 hash keys
- **Singleton**: `GlobalConfig` always ID=1 for server-wide settings
- **Facade**: Backend package abstracts complex database and embedding logic
- **Hub Pattern**: WebSocket connection management with username-based routing
- **JWT Validation**: OIDC auth via JWKS cache with 10-minute refresh interval

## Integration with Frontend

- Frontend fetches `/api/auth/config` for OIDC configuration
- Real-time embedding progress via WebSocket
- Config endpoints for user preferences
- Import endpoint for bulk data load
- Entity synchronization via WebSocket `update` messages

## Error Handling

- Huma response objects with status codes
- JSON error responses with `detail` field
- Custom error strings (e.g., `errorRecordNotFound`)
- Toast notifications on frontend for API errors
- Automatic token refresh on 401 responses