# Corrugation Frontend - Technical Documentation

This file provides guidance to developers working on the Corrugation frontend.

## Overview

Corrugation frontend is a Vue 3 + TypeScript single-page application for hierarchical entity management. The SPA uses Pinia for state management, Vue Router for navigation, and WebRTC for camera capture. AI-powered embeddings and search are handled by the backend server.

## Technology Stack

- Framework: Vue 3.5.32 (Composition API, `<script setup>` syntax)
- Build Tool: Vite 8.0.8
- Language: TypeScript 6.0.0 (noUncheckedIndexedAccess enabled)
- State Management: Pinia 3.0.4
- Routing: Vue Router 5.0.4
- Styling: TailwindCSS 4.2.4 + PostCSS
- Camera: WebRTC (`navigator.mediaDevices.getUserMedia`)
- Real-time: WebSocket for live entity updates

## Project Structure

The frontend source code lives under `src/` with the following organization:

- `api/`: API client functions, TypeScript types for backend records and entities, and Record-to-Entity mapping utilities
- `assets/`: Static assets including favicon, CSS, and images
- `components/`: Reusable Vue components for the UI
- `router/`: Vue Router configuration with route definitions and navigation guards
- `stores/`: Pinia stores for state management
- `utils/`: Utility functions (currently empty)
- `views/`: Page-level Vue components for each route

See the source files directly for the complete structure:
- [`src/api/index.ts`](src/api/index.ts) and [`src/api/types.ts`](src/api/types.ts) for API client
- [`src/router/index.ts`](src/router/index.ts) for router configuration
- [`src/stores/auth.ts`](src/stores/auth.ts), [`src/stores/entities.ts`](src/stores/entities.ts), [`src/stores/camera.ts`](src/stores/camera.ts), [`src/stores/toasts.ts`](src/stores/toasts.ts) for stores
- [`src/components/*.vue`](src/components/) for all components
- [`src/views/*.vue`](src/views/) for page views

## Key Stores

### auth.ts

See [`src/stores/auth.ts`](src/stores/auth.ts) for the complete implementation. Handles OIDC authentication with Authentik using PKCE flow. Stores tokens in localStorage and sets cookies with `SameSite=Strict`. Key methods include `fetchConfig()`, `startLogin()`, `handleCallback()`, `setToken()`, and `clearToken()`.

### entities.ts

See [`src/stores/entities.ts`](src/stores/entities.ts) for the complete implementation. Manages all records fetched from the API, entity maps computed from records, location trees for hierarchy navigation, and WebSocket connection for live updates. Contains search state including debounced search text, filter flags, and embedding progress tracking. Includes `nameMap` computed property that maps entity IDs to reference numbers or names for sorting and display.

### camera.ts

See [`src/stores/camera.ts`](src/stores/camera.ts) for the complete implementation. Handles WebRTC camera access with `getUserMedia()`, device enumeration and selection, camera lifecycle (open, capture, preview, retake, close), and keyboard shortcuts for different states. Persists device ID in localStorage. Detects device orientation for mobile cameras and handles rotation transformations.

### toasts.ts

See [`src/stores/toasts.ts`](src/stores/toasts.ts) for the complete implementation. Manages notification toasts with auto-dismiss behavior (5 seconds), type levels (info/warn/error), and methods for adding, updating, finalizing, and removing toasts.

## Routes

| Path | Component | Auth Required | Description |
|------|-----------|---------------|-------------|
| `/login` | LoginView | N/A | Trigger OIDC flow |
| `/callback` | CallbackView | N/A | OAuth2 code/callback handler |
| `/` | EntityView | If auth enabled | Main entity browser |

## API Client

See [`src/api/index.ts`](src/api/index.ts) for the complete implementation. Provides methods for all backend endpoints under `/api/`:
- Record operations: `getRecords()`, `createRecord()`, `updateRecord()`, `deleteRecord()`, `moveRecord()`, `patchRecord()`
- Search: `searchRecords()` with embedding and substring options (returns 207 for partial results)
- Filter syntax: `filter:missing-image` shows only entities without images, `filter:only-image` shows only entities with images
- Artifacts: `uploadArtifact()`, `deleteArtifact()`
- Embeddings: `getSearchEmbeddingProgress()` for indexing progress
- Utils: `nextReferenceNumber()` to get next available reference number, `getStoreVersion()`

The `apiFetch()` wrapper handles 401 redirects to `/login` and error toasts on all failures except 401 errors (which redirect silently). `withErrorToast()` wraps calls with automatic toast on failure.

## Build Configuration

- OutDir: `../dist` (served by backend)
- Proxy: `/api` → `http://localhost:8083`, `/ws` → WebSocket
- Env: `DEBUG` defined as `mode !== "production"`
- Icons: Auto-generated via `vite-plugin-favicon-generator`
- SSL: Enabled in dev via `@vitejs/plugin-basic-ssl` (configures `https: {}`)
- Path alias: `@/` → `src/`

See [`vite.config.ts`](vite.config.ts) for complete build configuration.

## TypeScript Setup

Extends `@vue/tsconfig/tsconfig.dom.json` with path alias `@/` → `src/`. Enables `noUncheckedIndexedAccess: true` for safer array/object access and strict null checks. Build-time type checking with `vue-tsc --build`.

See [`tsconfig.json`](tsconfig.json), [`tsconfig.app.json`](tsconfig.app.json), and [`tsconfig.node.json`](tsconfig.node.json) for TypeScript configuration.

## Component Patterns

See the individual component files for complete implementations:

### EntityCard
See [`src/components/EntityCard.vue`](src/components/EntityCard.vue). Displays entity metadata with inline editing mode, quick capture buttons (**P** for capture on entity, **⇧C** for capture new child, **⇧N** for create child entity), delete confirmation dialog, and move dialog with hierarchical location selector. Keyboard shortcuts: **Del** to delete, **M** to move, **Enter** to edit.

### SearchBar
See [`src/components/SearchBar.vue`](src/components/SearchBar.vue). Debounced input (500ms), filter toggles for world-scope (**G**), image embedding (**I**), text embedding (**W**), and string matching (**T**), keyboard shortcut hints (**/** for search, **?** for command palette), clear button.

### CameraModal
See [`src/components/CameraModal.vue`](src/components/CameraModal.vue). Fullscreen video preview with device selector, keyboard shortcuts that vary by state (**Enter** to capture/confirm, **R** to rotate, **C** to retake, **Escape** to close), orientation detection for mobile devices.

### CommandDialog
See [`src/components/CommandDialog.vue`](src/components/CommandDialog.vue). Keyboard command palette with fuzzy search and quick actions listed with their shortcuts. Lists all major keyboard commands including navigation, editing, capture, search toggles, and entity creation.

### ToastContainer
See [`src/components/ToastContainer.vue`](src/components/ToastContainer.vue). Bottom-right toast display with auto-dismiss fade animation, hover-to-pause, stacked vertical layout.

### BreadcrumbNav
See [`src/components/BreadcrumbNav.vue`](src/components/BreadcrumbNav.vue). Hierarchical navigation showing parent chain with click and drag-drop support. Allows navigation to parent entities and dragging entities to relocate them.

### NewEntityDialog
See [`src/components/NewEntityDialog.vue`](src/components/NewEntityDialog.vue). Modal for creating entities with fields for name, reference #, description, quantity, and images, plus reference collision detection that warns when reference number is already in use.

### QuickCaptureCard
See [`src/components/QuickCaptureCard.vue`](src/components/QuickCaptureCard.vue). Large clickable card for camera capture that creates new entities with minimal information (image artifact only).

### ArtifactImage
See [`src/components/ArtifactImage.vue`](src/components/ArtifactImage.vue). Lazy-loaded artifact images with ETag caching.

### KbdHint
See [`src/components/KbdHint.vue`](src/components/KbdHint.vue). Small keyboard hint component that displays keyboard shortcuts inline or with toggle visibility. Used throughout the UI for showing keyboard shortcuts.

### WelcomeItem
See [`src/components/WelcomeItem.vue`](src/components/WelcomeItem.vue). Welcome/item display component used in the EntityView for initial welcome message or entity items.

## Dependencies Summary

**Production**:
- vue (3.5.32)
- vue-router (5.0.4)
- pinia (3.0.4)
- @mdi/js, vue-material-design-icons (icons)

**Development**:
- vite (8.0.8)
- @vitejs/plugin-vue (6.0.6)
- @tailwindcss/postcss, tailwindcss (4.2.4)
- vue-tsc (3.2.6)
- vite-plugin-vue-devtools (8.1.1)
- vite-plugin-favicon-generator (1.0.2)
- @vitejs/plugin-basic-ssl (2.3.0)
- npm-run-all2 (8.0.4)

See [`package.json`](package.json) for complete dependency list.

## Frontend Architecture Details

### Router Guards
See [`src/router/index.ts`](src/router/index.ts) for complete implementation. Fetches auth config on first navigation, allows callback route unconditionally, redirects to `/login` if auth enabled and no token, and redirects to `/` if already authenticated on `/login`.

### WebSocket Connection
See [`src/stores/entities.ts`](src/stores/entities.ts) for `connectWS()` and `updateEmbeddingProgressForSearch()` methods. Auto-reconnects after 3s on close, passes token as query parameter, handles embedding progress messages, reload signals, and offline/online status.

### Search Debounce & Filtering
See [`src/components/SearchBar.vue`](src/components/SearchBar.vue) `handleSearchInput()` and [`src/stores/entities.ts`](src/stores/entities.ts) watch handler. Debounced on `searchtextpredebounce` changes (500ms), uses `debouncesearch()` to set `searchtext`. Filter syntax is parsed from the search text: `filter:missing-image` filters to only entities without images, `filter:only-image` filters to only entities with images. Shows progress toast during indexing when embeddings are incomplete.

## Error Handling

See [`src/api/index.ts`](src/api/index.ts) for error handling implementation. Extracts error messages from JSON `detail` field, shows toast on all API errors **except 401** (which redirects to `/login` silently), and logs silent failures to console. Camera errors fall back to toast notifications.

## Camera Behavior

See [`src/stores/camera.ts`](src/stores/camera.ts) `open()` method for implementation. Requests camera permission on first call, stores device ID in localStorage for persistence. The code relies on browser enforcement of HTTPS in production rather than explicit checks. Detects device orientation on mobile and handles automatic rotation. Camera fails silently if no device is found.

---

**See**
- [`../backend/AGENTS.md`](../backend/AGENTS.md) for backend-specific guidance