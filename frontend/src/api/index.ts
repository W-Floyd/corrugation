import type { BackendRecord, RecordBody } from "./types";
import router from "../router";
import { useAuthStore } from "../stores/auth";
import { useToast } from "primevue/usetoast";
import { DEFAULT_TOAST_LIFE } from "../stores/constants";

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
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Error",
      detail: message,
      life: DEFAULT_TOAST_LIFE,
    });
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
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Error",
      detail: e instanceof Error ? e.message : String(e),
      life: DEFAULT_TOAST_LIFE,
    });
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

  async getGlobalConfig(): Promise<{
    logLevel: string;
    backfillRecordEmbeddingsOnStart: boolean;
    backfillArtifactEmbeddingsOnStart: boolean;
    backfillArtifactOwnersOnStart: boolean;
    allowLocalUsernameLogin: boolean;
    infinityTextModel: string;
    infinityImageModel: string;
    infinityTextQueryPrefix: string;
    infinityTextDocumentPrefix: string;
  }> {
    const response = await apiFetch("/api/config/global");
    return response.json();
  },

  async updateGlobalConfig(config: {
    logLevel: string;
    backfillRecordEmbeddingsOnStart: boolean;
    backfillArtifactEmbeddingsOnStart: boolean;
    backfillArtifactOwnersOnStart: boolean;
    allowLocalUsernameLogin: boolean;
    infinityTextModel: string;
    infinityImageModel: string;
    infinityTextQueryPrefix: string;
    infinityTextDocumentPrefix: string;
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
  }> {
    const response = await apiFetch("/api/config/user");
    return response.json();
  },

  async updateUserConfig(config: {
    infinityTextModel?: string | null;
    infinityImageModel?: string | null;
    infinityTextQueryPrefix?: string | null;
    infinityTextDocumentPrefix?: string | null;
  }): Promise<void> {
    await apiFetch("/api/config/user", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });
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
