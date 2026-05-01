import { defineStore } from "pinia";
import { ref, computed, watch } from "vue";
import type { BackendRecord } from "@/api/types";
import { api } from "@/api";
import { useToast } from "primevue/usetoast";
import { DEFAULT_TOAST_LIFE } from "./constants";
const toast = useToast();

export const useRecordsStore = defineStore("records", () => {
  const currentRecord = ref<number>(0);

  // All records fetched globally — kept in sync via WebSocket
  const allRecords = ref<BackendRecord[]>([]);

  const locationtree = ref<number[]>([0]);

  const searchtextpredebounce = ref("");
  const searchtext = ref("");
  const moveSearchtext = ref("");
  const selectedRecordId = ref<number | null>(null);
  const filterworld = ref(false);
  const searching = ref(false);
  const searchImage = ref(true);
  const searchTextEmbedded = ref(true);
  const searchTextSubstring = ref(true);
  const apiSearchResults = ref<BackendRecord[]>([]);
  const apiSearchResultsPartial = ref<boolean>(false);
  const apiSearchScores = ref<
    Record<number, { image?: number; text?: number }>
  >({});
  const filterToMissingImage = ref(false);
  const filterToOnlyImage = ref(false);

  const isLoading = ref(true);

  // Search embedding progress tracking
  const searchProgress = ref<{
    record: {
      complete: number[];
      pending: number[];
    };
    artifact: {
      complete: number[];
      pending: number[];
    };
  } | null>(null);

  let ws: WebSocket | null = null;

  // All records indexed by ID
  const recordById = computed<Record<number, BackendRecord>>(() => {
    const m: Record<number, BackendRecord> = {};
    for (const r of allRecords.value) m[r.ID] = r;
    return m;
  });

  const recordMap = computed<Record<number, BackendRecord>>(() => {
    const m: Record<number, BackendRecord> = {};
    for (const r of allRecords.value) m[r.ID] = r;
    return m;
  });

  const nameMap = computed<Record<number, string>>(() => {
    const m: Record<number, string> = { 0: "World" };
    for (const r of allRecords.value) {
      m[r.ID] = r.ReferenceNumber ?? r.Title ?? String(r.ID);
    }
    return m;
  });

  // Derived from allRecords — no extra fetch needed
  function buildLocationTree(recordId: number): number[] {
    if (recordId === 0) return [0];
    const tree: number[] = [];
    let cur: number | undefined = recordId;
    while (cur !== undefined && cur !== 0) {
      tree.push(cur);
      cur = recordById.value[cur]?.ParentID ?? undefined;
    }
    tree.push(0);
    tree.reverse();
    return tree;
  }

  async function reload(): Promise<void> {
    try {
      allRecords.value = await api.getRecords(0, { global: true });
      locationtree.value = buildLocationTree(currentRecord.value);
      isLoading.value = false;
      const existingIds = new Set(allRecords.value.map((r) => r.ID));
      apiSearchResults.value = apiSearchResults.value.filter((e) =>
        existingIds.has(e.ID),
      );
    } catch (e) {
      console.error("reload failed:", e);
    }
  }

  type PartialScope = {
    scopeId: number | undefined;
    searchImage: boolean;
    searchTextEmbedded: boolean;
  };

  function clearProgressToast(): void {
    // No toast to clear for PrimeVue implementation
  }

  async function fetchAndUpdateProgress(scope: PartialScope): Promise<void> {
    try {
      const progress = await api.getSearchEmbeddingProgress({
        id: scope.scopeId,
        global: scope.scopeId == null,
        childrenDepth: scope.scopeId != null ? -1 : undefined,
        searchImage: scope.searchImage,
        searchTextEmbedded: scope.searchTextEmbedded,
      });
      // Skip if search already complete
      if (
        progress.record?.pending.length === 0 &&
        progress.artifact?.pending.length === 0
      ) {
        // Don't recreate toast - just exit
        if (DEBUG)
          console.log("[records] embedding already complete, skipping update");
        return;
      }
      // Skip if search already complete
      if (DEBUG)
        console.log("[records] getSearchEmbeddingProgress result:", progress);
      searchProgress.value = progress;
      if (
        progress.record?.pending.length === 0 &&
        progress.artifact?.pending.length === 0
      ) {
        const toast = useToast();
        toast.add({
          severity: "info",
          summary: "Search Complete",
          detail: "Embeddings ready — re-run search for complete results",
          life: DEFAULT_TOAST_LIFE,
        });
      }
    } catch {
      // ignore — will retry on next embedding_progress message
    }
  }

  function connectWS(): void {
    const protocol = location.protocol === "https:" ? "wss" : "ws";
    const token = localStorage.getItem("auth_token");
    const url = `${protocol}://${location.host}/ws${token ? `?token=${encodeURIComponent(token)}` : ""}`;
    if (DEBUG) console.log("[records] connectWS", url);
    ws = new WebSocket(url);
    ws.onopen = () => {
      reload();
    };
    ws.onmessage = async (e) => {
      if (typeof e.data !== "string") {
        reload();
        return;
      }
      if (e.data.startsWith("embedding_progress")) {
        updateEmbeddingProgressForSearch(e.data);
      } else if (e.data === "embedding_server_offline") {
        toast.add({
          severity: "warn",
          summary: "Embedding server offline",
          detail: "Indexing will resume automatically",
          life: DEFAULT_TOAST_LIFE,
        });
      } else if (e.data === "embedding_server_online") {
        toast.add({
          severity: "success",
          summary: "Embedding server online",
          detail: "Indexing services restored",
          life: DEFAULT_TOAST_LIFE,
        });
      } else {
        toast.add({
          severity: "info",
          summary: "Record Updated",
          detail: "Record updated successfully",
          life: DEFAULT_TOAST_LIFE,
        });
        reload();
      }
    };

    ws.onclose = () => {
      setTimeout(() => connectWS(), 3000);
    };
  }

  function updateEmbeddingProgressForSearch(fullMessage: string): void {
    if (!searchProgress.value) return;
    const jobType = fullMessage.includes(":record:")
      ? ("record" as "record" | "artifact")
      : ("artifact" as "record" | "artifact");
    const progressObj = searchProgress.value[jobType];
    if (!progressObj) return;
    const arr = progressObj.pending;
    if (!arr.length) return;
    const idMatch = fullMessage.match(/:(\d+)$/);
    if (!idMatch || !idMatch[1]) return;
    const id = parseInt(idMatch[1], 10);

    if (arr.includes(id)) {
      arr.splice(arr.indexOf(id), 1);
      progressObj.complete.push(id);
    } else {
      arr.push(id);
    }
    searchProgress.value = { ...searchProgress.value, [jobType]: progressObj };
  }

  async function setCurrentRecord(recordId: number): Promise<void> {
    if (isNaN(recordId)) recordId = 0;
    currentRecord.value = recordId;
    locationtree.value = buildLocationTree(recordId);
    import("../router").then(({ default: router }) => {
      router.push({ query: recordId === 0 ? {} : { record: recordId } });
    });
    searchtext.value = "";
  }

  async function searchByImage(file: File): Promise<void> {
    try {
      // Set a placeholder search term to trigger display of image search results
      searchtextpredebounce.value = "🔍";
      searchtext.value = "🔍";
      searching.value = true;
      filterToMissingImage.value = false;
      filterToOnlyImage.value = false;
      apiSearchResults.value = [];
      apiSearchResultsPartial.value = false;
      apiSearchScores.value = {};
      const { results, partial } = await api.searchByImage(file);
      apiSearchResults.value = results.map((r) => r.record);
      // Store image scores for display on cards
      for (const r of results) {
        apiSearchScores.value[r.record.ID] = { image: r.imageScore };
      }
      apiSearchResultsPartial.value = partial;
      searching.value = false;
    } catch (e) {
      console.error("Image search failed:", e);
      searching.value = false;
      apiSearchResults.value = [];
      apiSearchResultsPartial.value = false;
    }
  }

  function readname(recordId: number): string {
    if (recordId === 0) return "World";
    return nameMap.value[recordId] ?? String(recordId);
  }

  function hasChildren(recordId: number): boolean {
    return allRecords.value.some((r) => (r.ParentID ?? 0) === recordId);
  }

  function listChildLocations(recordId: number): number[] {
    return allRecords.value
      .filter((r) => (r.ParentID ?? 0) === recordId)
      .map((r) => r.ID)
      .sort((a, b) => {
        const na = nameMap.value[a]?.toLowerCase() ?? "";
        const nb = nameMap.value[b]?.toLowerCase() ?? "";
        return na.localeCompare(nb, undefined, { numeric: true });
      });
  }

  function load(locationId: number, searchTextVal: string): BackendRecord[] {
    if (searchTextVal.trim()) {
      let results = [...apiSearchResults.value];
      if (filterToMissingImage.value) {
        results = results.filter(
          (e) => !e.Artifacts || e.Artifacts.length === 0,
        );
      } else if (filterToOnlyImage.value) {
        results = results.filter((e) => e.Artifacts && e.Artifacts.length > 0);
      }
      return results;
    }
    return allRecords.value
      .filter((r) => (r.ParentID ?? 0) === locationId)
      .sort((a, b) => {
        const na = (a.Title ?? "").toLowerCase();
        const nb = (b.Title ?? "").toLowerCase();
        return na.localeCompare(nb, undefined, { numeric: true });
      });
  }

  function debouncesearch(): void {
    searchtext.value = searchtextpredebounce.value;
  }

  watch(
    [
      searchtext,
      filterworld,
      currentRecord,
      searchImage,
      searchTextEmbedded,
      searchTextSubstring,
    ],
    async ([text]) => {
      if (!text.trim()) {
        searching.value = false;
        apiSearchResults.value = [];
        apiSearchScores.value = {};
        filterToMissingImage.value = false;
        filterToOnlyImage.value = false;
        clearProgressToast();
        return;
      }

      let query = text;
      filterToMissingImage.value = query.includes("filter:missing-image");
      filterToOnlyImage.value =
        !filterToMissingImage.value && query.includes("filter:only-image");
      query = query
        .replace("filter:missing-image", "")
        .replace("filter:only-image", "")
        .trim();

      clearProgressToast();
      // Clear old results immediately to prevent stale data during load
      apiSearchResults.value = [];
      apiSearchScores.value = {};
      searching.value = true;
      try {
        if (query) {
          const scopeId =
            !filterworld.value && currentRecord.value !== 0
              ? currentRecord.value
              : undefined;
          const { results, partial } = await api.searchRecords(query, {
            parentId: scopeId,
            searchImage: searchImage.value,
            searchTextEmbedded: searchTextEmbedded.value,
            searchTextSubstring: searchTextSubstring.value,
          });
          if (partial) {
            const toast = useToast();
            toast.add({
              severity: "warn",
              summary: "Embeddings Pending",
              detail: "Indexing embeddings — results may be incomplete",
              life: 0,
            });
            fetchAndUpdateProgress({
              scopeId,
              searchImage: searchImage.value,
              searchTextEmbedded: searchTextEmbedded.value,
            });
          }
          apiSearchResults.value = results.map((r) => r.record);
          const scores: Record<number, { image?: number; text?: number }> = {};
          for (const r of results) {
            scores[r.record.ID] = { image: r.imageScore, text: r.textScore };
          }
          apiSearchScores.value = scores;
        } else {
          apiSearchResults.value = [];
        }
      } catch (e) {
        console.error("Search failed:", e);
        apiSearchResults.value = [];
      } finally {
        searching.value = false;
      }
    },
  );

  // Clear any pending toasts on record change
  watch(currentRecord, () => {
    // No toast cleanup needed with PrimeVue implementation
  });

  // Update progress toast based on searchProgress state
  watch(
    () => searchProgress.value,
    (progress) => {
      if (!progress) return;
      const record = progress.record;
      const artifact = progress.artifact;
      if (!record || !artifact) return;

      const total =
        (record?.complete?.length || 0) +
        (record?.pending?.length || 0) +
        (artifact?.complete?.length || 0) +
        (artifact?.pending?.length || 0);
      const pending =
        (record?.pending?.length || 0) + (artifact?.pending?.length || 0);

      if (pending === 0) {
        if (DEBUG) console.log("[records] embedding complete");
        const toast = useToast();
        toast.add({
          severity: "success",
          summary: "Embeddings Ready",
          detail: "Embeddings ready",
          life: DEFAULT_TOAST_LIFE,
        });
        return;
      }

      if (DEBUG) {
        console.log(
          "[records] embedding progress:",
          total,
          "pending:",
          pending,
        );
      }

      // Show progress toast
      const toast = useToast();
      toast.add({
        severity: "warn",
        summary: "Embedding Progress",
        detail: `Indexing ${total} embeddings (${pending} pending)`,
        life: 0,
      });
    },
    { deep: true },
  );

  return {
    currentRecord,
    allRecords,
    nameMap,
    recordMap,
    locationtree,
    searchtextpredebounce,
    searchtext,
    moveSearchtext,
    selectedRecordId,
    filterworld,
    searching,
    searchImage,
    searchTextEmbedded,
    searchTextSubstring,
    apiSearchResults,
    apiSearchResultsPartial,
    apiSearchScores,
    filterToMissingImage,
    filterToOnlyImage,
    isLoading,
    searchProgress,
    reload,
    connectWS,
    setCurrentRecord,
    searchByImage,
    readname,
    hasChildren,
    listChildLocations,
    load,
    debouncesearch,
  };
});
