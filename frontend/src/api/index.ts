import type { BackendRecord, RecordBody } from "./types";
import router from "../router";
import { useAuthStore } from "../stores/auth";
import { useToastsStore } from "../stores/toasts";

export async function apiFetch(
  url: string,
  options: RequestInit = {},
): Promise<Response> {
  const token = localStorage.getItem("auth_token");
  const headers = new Headers(options.headers as HeadersInit);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(url, { ...options, headers });

  if (response.status === 401) {
    const currentRoute = router.currentRoute.value.name;
    if (DEBUG)
      console.warn(
        "[apiFetch] 401 on",
        url,
        "current route:",
        currentRoute,
        new Error().stack?.split("\n")[2]?.trim(),
      );
    if (currentRoute !== "callback") {
      useAuthStore().clearToken();
      router.push({ name: "login" });
      throw new Error("Unauthorized");
    }
    throw new Error("Unauthorized");
  }

  if (!response.ok) {
    const body = await response.text();
    let message = body || `HTTP ${response.status}`;
    try {
      const parsed = JSON.parse(body);
      if (parsed.detail) {
        message = parsed.title
          ? `${parsed.title}: ${parsed.detail}`
          : parsed.detail;
      } else if (parsed.title) {
        message = parsed.title;
      }
    } catch {
      /* not JSON, use raw body */
    }
    useToastsStore().add(message);
    throw new Error(message);
  }

  return response;
}

export async function withErrorToast<T>(
  fn: () => Promise<T>,
): Promise<T | undefined> {
  try {
    return await fn();
  } catch (e) {
    useToastsStore().add(e instanceof Error ? e.message : String(e));
    return undefined;
  }
}

export const api = {
  // Fetch records at a location. id=0 → top-level. global=true → all.
  async getRecords(
    locationId: number,
    opts: {
      childrenDepth?: number;
      parentDepth?: number;
      global?: boolean;
      timestamps?: boolean;
    } = {},
  ): Promise<BackendRecord[]> {
    const params = new URLSearchParams();
    if (opts.global) {
      params.set("global", "true");
    } else {
      params.set("id", String(locationId));
    }
    if (opts.childrenDepth !== undefined)
      params.set("childrenDepth", String(opts.childrenDepth));
    if (opts.parentDepth !== undefined)
      params.set("parentDepth", String(opts.parentDepth));
    if (opts.timestamps) params.set("timestamps", "true");
    const response = await apiFetch(`/api/records?${params}`);
    return response.json();
  },

  async createRecord(body: RecordBody): Promise<BackendRecord> {
    const response = await apiFetch("/api/record", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return response.json();
  },

  async updateRecord(id: number, body: RecordBody): Promise<BackendRecord> {
    const response = await apiFetch(`/api/record/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return response.json();
  },

  async deleteRecord(id: number): Promise<void> {
    await apiFetch(`/api/record/${id}`, { method: "DELETE" });
  },

  async moveRecord(id: number, locationId: number): Promise<void> {
    await apiFetch(`/api/record/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/merge-patch+json" },
      body: JSON.stringify({
        ParentID: locationId,
      }),
    });
  },

  async patchRecord(id: number, body: Partial<RecordBody>): Promise<void> {
    await apiFetch(`/api/record/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/merge-patch+json" },
      body: JSON.stringify(body),
    });
  },

  async searchRecords(
    query: string,
    opts: {
      parentId?: number;
      searchImage?: boolean;
      searchTextEmbedded?: boolean;
      searchTextSubstring?: boolean;
      searchSuggested?: boolean;
    } = {},
  ): Promise<{
    results: {
      record: BackendRecord;
      imageScore?: number;
      textScore?: number;
    }[];
    partial: boolean;
  }> {
    const params = new URLSearchParams({ search: query });
    if (opts.parentId != null) {
      params.set("id", String(opts.parentId));
      params.set("childrenDepth", "-1");
    } else {
      params.set("global", "true");
    }
    if (opts.searchImage !== false) params.set("searchImage", "true");
    if (opts.searchTextEmbedded !== false)
      params.set("searchTextEmbedded", "true");
    if (opts.searchTextSubstring !== false)
      params.set("searchTextSubstring", "true");
    if (opts.searchSuggested !== false) params.set("searchSuggested", "true");
    const response = await apiFetch(`/api/records?${params}`);
    const partial = response.status === 207;
    const records: BackendRecord[] = await response.json();
    return {
      partial,
      results: records.map((r) => ({
        record: r as BackendRecord,
        imageScore: r.SearchConfidenceImage,
        textScore: r.SearchConfidenceText,
      })),
    };
  },

  async searchByImage(file: File): Promise<{
    results: { record: BackendRecord; imageScore?: number }[];
    partial: boolean;
  }> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await apiFetch("/api/search/image", {
      method: "POST",
      body: formData,
    });
    const partial = response.status === 207;
    const records: BackendRecord[] = await response.json();
    return {
      partial,
      results: records.map((r) => ({
        record: r as BackendRecord,
        imageScore: r.SearchConfidenceImage,
        textScore: r.SearchConfidenceText,
      })),
    };
  },

  async uploadArtifact(file: File): Promise<number> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await apiFetch("/api/artifact", {
      method: "POST",
      body: formData,
    });
    const id = await response.json();
    return typeof id === "number" ? id : parseInt(id, 10);
  },

  async deleteArtifact(id: number): Promise<void> {
    await apiFetch(`/api/artifact/${id}`, { method: "DELETE" });
  },

  async suggestFromImage(
    file: File,
  ): Promise<{ name: string; description: string; quantity?: number }> {
    const formData = new FormData();
    formData.append("file", file);
    const response = await apiFetch("/api/suggest", {
      method: "POST",
      body: formData,
    });
    return response.json();
  },

  // Next available reference number not held by any labeled record
  async nextReferenceNumber(excludeIDs?: number[]): Promise<number> {
    const params = new URLSearchParams();
    if (excludeIDs != null) {
      params.set("excludeIDs", excludeIDs.toString());
    }
    const response = await apiFetch(`/api/records/nextref?${params}`);
    return response.json();
  },

  async getSearchEmbeddingProgress(opts: {
    id?: number;
    global?: boolean;
    childrenDepth?: number;
    searchImage?: boolean;
    searchTextEmbedded?: boolean;
  }): Promise<{
    record: { complete: number[]; pending: number[] };
    artifact: { complete: number[]; pending: number[] };
  }> {
    const params = new URLSearchParams();
    if (opts.global) params.set("global", "true");
    else if (opts.id != null) params.set("id", String(opts.id));
    if (opts.childrenDepth != null)
      params.set("childrenDepth", String(opts.childrenDepth));
    if (opts.searchImage) params.set("searchImage", "true");
    if (opts.searchTextEmbedded) params.set("searchTextEmbedded", "true");
    const response = await apiFetch(
      `/api/embeddings/search-progress?${params}`,
    );
    return response.json();
  },

  async getStoreVersion(): Promise<number> {
    const response = await apiFetch("/api/store/version");
    return response.json();
  },

  // Local username login for testing (when AllowLocalUsernameLogin is enabled)
  async login(username: string): Promise<{ username: string }> {
    const response = await apiFetch("/api/auth/local/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username }),
    });
    return response.json();
  },

  async getCapabilities(): Promise<{
    barcodeFormats: { value: string; label: string }[];
  }> {
    const response = await apiFetch("/api/capabilities");
    return response.json();
  },

  async getGlobalConfig(): Promise<{
    logLevel: string;
    backfillLegacyEmbeddingsOnStart: boolean;
    backfillRecordEmbeddingsOnStart: boolean;
    backfillArtifactEmbeddingsOnStart: boolean;
    backfillArtifactOwnersOnStart: boolean;
    backfillSuggestionsOnStart: boolean;
    allowLocalUsernameLogin: boolean;
    infinityTextModel: string;
    infinityImageModel: string;
    infinityTextQueryPrefix: string;
    infinityTextDocumentPrefix: string;
    enabledBarcodeFormats: string[];
    maximumEmbeddingDimensions?: number;
    ollamaAddress: string;
    ollamaVisionModel: string;
    ollamaNumCtx: number;
    ollamaImageMaxDim: number;
    ollamaSuggestPrompt: string;
  }> {
    const response = await apiFetch("/api/config/global");
    return response.json();
  },

  async updateGlobalConfig(config: {
    logLevel: string;
    backfillLegacyEmbeddingsOnStart: boolean;
    backfillRecordEmbeddingsOnStart: boolean;
    backfillArtifactEmbeddingsOnStart: boolean;
    backfillArtifactOwnersOnStart: boolean;
    backfillSuggestionsOnStart: boolean;
    allowLocalUsernameLogin: boolean;
    infinityTextModel: string;
    infinityImageModel: string;
    infinityTextQueryPrefix: string;
    infinityTextDocumentPrefix: string;
    enabledBarcodeFormats: string[];
    maximumEmbeddingDimensions?: number | null;
    ollamaAddress: string;
    ollamaVisionModel: string;
    ollamaNumCtx: number;
    ollamaImageMaxDim: number;
    ollamaSuggestPrompt: string;
  }): Promise<void> {
    await apiFetch("/api/config/global", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
  },

  async getUserConfig(): Promise<{
    infinityTextModel?: string;
    infinityImageModel?: string;
    infinityTextQueryPrefix?: string;
    infinityTextDocumentPrefix?: string;
    enabledBarcodeFormats?: string[];
    maximumEmbeddingDimensions?: number;
    ollamaAddress?: string;
    ollamaVisionModel?: string;
    ollamaNumCtx?: number;
    ollamaImageMaxDim?: number;
    ollamaSuggestPrompt?: string;
  }> {
    const response = await apiFetch("/api/config/user");
    return response.json();
  },

  async updateUserConfig(config: {
    infinityTextModel?: string | null;
    infinityImageModel?: string | null;
    infinityTextQueryPrefix?: string | null;
    infinityTextDocumentPrefix?: string | null;
    enabledBarcodeFormats?: string[] | null;
    maximumEmbeddingDimensions?: number | null;
    ollamaAddress?: string | null;
    ollamaVisionModel?: string | null;
    ollamaNumCtx?: number | null;
    ollamaImageMaxDim?: number | null;
    ollamaSuggestPrompt?: string | null;
  }): Promise<void> {
    await apiFetch("/api/config/user", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
  },

  async getEmbeddingJobs(
    opts: {
      all?: boolean;
      status?: string;
      limit?: number;
      offset?: number;
    } = {},
  ): Promise<{
    jobs: {
      id: number;
      jobType: string;
      targetID: number;
      username: string;
      status: string;
      errorMsg?: string;
      retryCount: number;
      embedModel: string;
      dimensions?: number;
      source: string;
      createdAt: string;
      updatedAt: string;
    }[];
    total: number;
  }> {
    const params = new URLSearchParams();
    if (opts.all) params.set("all", "true");
    if (opts.status) params.set("status", opts.status);
    if (opts.limit != null) params.set("limit", String(opts.limit));
    if (opts.offset != null) params.set("offset", String(opts.offset));
    const response = await apiFetch(`/api/embeddings/jobs?${params}`);
    return response.json();
  },

  async resetStuckEmbeddingJobs(): Promise<void> {
    await apiFetch("/api/embeddings/jobs/reset", { method: "POST" });
  },

  async deleteBulkEmbeddingJobs(status: string, all?: boolean): Promise<void> {
    const params = new URLSearchParams({ status });
    if (all) params.set("all", "true");
    await apiFetch(`/api/embeddings/jobs?${params}`, { method: "DELETE" });
  },

  async deleteEmbeddingJob(id: number): Promise<void> {
    await apiFetch(`/api/embeddings/jobs/${id}`, { method: "DELETE" });
  },

  async invalidateUserEmbeddings(): Promise<void> {
    await apiFetch("/api/embeddings/user", { method: "DELETE" });
  },

  async getSuggestionJobs(
    opts: {
      all?: boolean;
      status?: string;
      limit?: number;
      offset?: number;
    } = {},
  ): Promise<{
    jobs: {
      id: number;
      artifactID: number;
      ollamaModel: string;
      username: string;
      status: string;
      errorMsg?: string;
      retryCount: number;
      source: string;
      createdAt: string;
      updatedAt: string;
    }[];
    total: number;
  }> {
    const params = new URLSearchParams();
    if (opts.all) params.set("all", "true");
    if (opts.status) params.set("status", opts.status);
    if (opts.limit != null) params.set("limit", String(opts.limit));
    if (opts.offset != null) params.set("offset", String(opts.offset));
    const response = await apiFetch(`/api/suggestions/jobs?${params}`);
    return response.json();
  },

  async resetStuckSuggestionJobs(): Promise<void> {
    await apiFetch("/api/suggestions/jobs/reset", { method: "POST" });
  },

  async deleteBulkSuggestionJobs(status: string, all?: boolean): Promise<void> {
    const params = new URLSearchParams({ status });
    if (all) params.set("all", "true");
    await apiFetch(`/api/suggestions/jobs?${params}`, { method: "DELETE" });
  },

  async deleteSuggestionJob(id: number): Promise<void> {
    await apiFetch(`/api/suggestions/jobs/${id}`, { method: "DELETE" });
  },

  async getArtifactSuggestion(id: number): Promise<{
    status: "ready" | "pending" | "stale";
    name?: string;
    description?: string;
    quantity?: number;
    ollamaModel?: string;
  } | null> {
    try {
      const response = await apiFetch(`/api/artifact/${id}/suggestion`);
      return response.json();
    } catch {
      return null;
    }
  },

  async getBackfillPreview(): Promise<{
    legacyEmbeddings: number;
    records: number;
    artifacts: number;
    suggestions: number;
  }> {
    const response = await apiFetch("/api/backfill/preview");
    return response.json();
  },

  async runSuggestionsBackfill(): Promise<void> {
    await apiFetch("/api/backfill/suggestions", { method: "POST" });
  },

  async pullOllamaModel(model: string): Promise<void> {
    await apiFetch("/api/ollama/pull", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ model }),
    });
  },

  async getOllamaModels(): Promise<string[]> {
    try {
      const response = await apiFetch("/api/ollama/models");
      return response.json();
    } catch {
      return [];
    }
  },

  async runRecordBackfill(): Promise<void> {
    await apiFetch("/api/backfill/records", { method: "POST" });
  },

  async runArtifactBackfill(): Promise<void> {
    await apiFetch("/api/backfill/artifacts", { method: "POST" });
  },

  async runLegacyEmbeddingsBackfill(): Promise<void> {
    await apiFetch("/api/backfill/legacy-embeddings", { method: "POST" });
  },

  async getUsers(): Promise<
    { id: number; username: string; isAdmin: boolean }[]
  > {
    const response = await apiFetch("/api/users");
    return response.json();
  },

  async setUserAdmin(username: string, isAdmin: boolean): Promise<void> {
    await apiFetch(`/api/users/${encodeURIComponent(username)}/admin`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ isAdmin }),
    });
  },
};

export default api;
