# Corrugation - Project Overview

## Architecture

Corrugation is a hierarchical record management system with computer vision capabilities. The backend serves REST APIs with AI-powered search, while the frontend provides a Vue-based interactive interface for record organization and camera capture.

## Monorepo Structure

```
corrugation/
├── backend/      # Go backend API server
│   └── AGENTS.md # Backend-specific guidance
├── frontend/     # Vue 3 + TypeScript SPA
│   └── AGENTS.md # Frontend-specific guidance
├── infinity/     # AI embedding server (external dependency)
├── bruno/        # API testing collection
└── AGENTS.md     # This file (top-level routing guide)
```

## Quick Navigation

### When to read `backend/AGENTS.md`

Work on backend/API tasks:
- API endpoint implementation or modification
- Database schema changes (SQLite/GORM models)
- Embedding system (Infinity integration, vector search)
- Authentication (OIDC/JWT configuration)
- Server configuration (CLI flags, environment variables)
- Workers and concurrency (embedding queue, WebSocket broadcast)
- Migration scripts or data import/export

**Key files**: `corrugation/main.go`, `corrugation/backend/handlers.go`, `corrugation/backend/record*.go`, `corrugation/backend/artifact*.go`, `corrugation/backend/embedding*.go`, `corrugation/backend/db.go`, `corrugation/backend/constants.go`, `corrugation/backend/import.go`

### When to read `frontend/AGENTS.md`

Work on frontend/UI tasks:
- Vue component development
- API client integration
- State management (Pinia stores)
- Routing configuration
- Camera features
- TypeScript types and API mappings

**Key files**: `frontend/src/stores/records.ts`, `frontend/src/stores/auth.ts`, `frontend/src/api/*.ts`, `frontend/src/components/*.vue`

### When to reference this file (`AGENTS.md`)

Use this top-level file for:
- Project architecture questions
- Cross-cutting concerns (how frontend/backend interact)
- Understanding the overall system design
- API documentation routing

## System Integration

### API Communication
- REST endpoints under `/api/*` (backend serves on `:8083`)
- WebSocket at `/ws` for real-time updates
- Proxy configured in Vite (`/api` → backend, `/ws` → WebSocket)

### Authentication Flow
1. Frontend fetches `/api/auth/config` for OIDC endpoints
2. PKCE OAuth redirect to Authentik
3. Callback exchanges code for token
4. Token stored in localStorage + httpOnly cookie
5. Backend validates JWT via JWKS cache

### Embedding Pipeline
1. Record/Artifact changes enqueue `EmbeddingJob`
2. Worker pool processes jobs (default 4 concurrency)
3. Infinity server generates vectors (CLIP image, BGE text)
4. Results cached in `embeddings` table
5. WebSocket broadcast progress to frontend

### Database
- SQLite with WAL mode for concurrent reads
- GORM models: `Record`, `Artifact`, `Embedding`, `User`
- Config stored in singleton `GlobalConfig` (ID=1)

## Build Output
- Frontend builds to `../dist` (served by backend)
- Backend compiled binary: `main`
- Data directory: `./data/db.sqlite` (created by `--data ./data` flag)

## Getting Started

### Backend
```bash
cd corrugation
go run main.go --port 8083 --data ./data
```

### Frontend
```bash
cd corrugation/frontend
npm run dev  # proxies to backend
```

---

**See**
- [`backend/AGENTS.md`](backend/AGENTS.md) for backend-specific guidance
- [`frontend/AGENTS.md`](frontend/AGENTS.md) for frontend-specific guidance
