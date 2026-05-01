import { defineStore } from "pinia";
import { ref, computed, watch } from "vue";
import type { Entity, BackendRecord } from "@/api/types";
import { recordToEntity } from "@/api/types";
import { api } from "@/api";
import { useToastsStore } from "@/stores/toasts";
import { useAuthStore } from "@/stores/auth";

export const useEntitiesStore = defineStore("entities", () => {
  const currentEntity = ref<number>(0);

  // All records fetched globally — kept in sync via WebSocket
  const allRecords = ref<BackendRecord[]>([]);

  const locationtree = ref<number[]>([0]);

  const searchtextpredebounce = ref("");
  const searchtext = ref("");
  const moveSearchtext = ref("");
  const selectedEntityId = ref<number | null>(null);
  const filterworld = ref(false);
  const searching = ref(false);
  const searchImage = ref(true);
  const searchTextEmbedded = ref(true);
  const searchTextSubstring = ref(true);
  const apiSearchResults = ref<Entity[]>([]);
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
  let offlineToastId: number | null = null;

  // All records indexed by ID
  const recordById = computed<Record<number, BackendRecord>>(() => {
    const m: Record<number, BackendRecord> = {};
    for (const r of allRecords.value) m[r.ID] = r;
    return m;
  });

  const entityMap = computed<Record<number, Entity>>(() => {
    const m: Record<number, Entity> = {};
    for (const r of allRecords.value) m[r.ID] = recordToEntity(r);
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
  function buildLocationTree(entityId: number): number[] {
    if (entityId === 0) return [0];
    const tree: number[] = [];
    let cur: number | undefined = entityId;
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
      locationtree.value = buildLocationTree(currentEntity.value);
      isLoading.value = false;
      const existingIds = new Set(allRecords.value.map((r) => r.ID));
      apiSearchResults.value = apiSearchResults.value.filter((e) =>
        existingIds.has(e.id),
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
  let partialScope: PartialScope | null = null;
  let progressToastId: number | null = null;

  function clearProgressToast(): void {
    if (progressToastId !== null) {
      useToastsStore().remove(progressToastId);
      progressToastId = null;
    }
    partialScope = null;
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
        DEBUG &&
          console.log("[entities] embedding already complete, skipping update");
        return;
      }
      // Skip if toast already finalized
      if (progressToastId === null) {
        DEBUG && console.log("[entities] progressToastId null, creating toast");
        progressToastId = useToastsStore().add(
          "Indexing embeddings...",
          "warn",
          true,
        );
      }
      DEBUG &&
        console.log("[entities] getSearchEmbeddingProgress result:", progress);
      searchProgress.value = progress;
      if (
        progress.record?.pending.length === 0 &&
        progress.artifact?.pending.length === 0
      ) {
        useToastsStore().update(
          progressToastId,
          "Embeddings ready — re-run search for complete results",
          "info",
        );
        useToastsStore().finalize(progressToastId);
        progressToastId = null;
        partialScope = null;
      }
    } catch {
      // ignore — will retry on next embedding_progress message
    }
  }

  function connectWS(): void {
    const protocol = location.protocol === "https:" ? "wss" : "ws";
    const token = localStorage.getItem("auth_token");
    const url = `${protocol}://${location.host}/ws${token ? `?token=${encodeURIComponent(token)}` : ""}`;
    DEBUG && console.log("[entities] connectWS", url);
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
        if (offlineToastId === null) {
          offlineToastId = useToastsStore().add(
            "Embedding server is offline — indexing will resume automatically",
            "warn",
            true,
          );
        }
      } else if (e.data === "embedding_server_online") {
        if (offlineToastId !== null) {
          useToastsStore().remove(offlineToastId);
          offlineToastId = null;
          useToastsStore().add("Embedding server is online", "success");
        }
      } else {
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

  async function setCurrentEntity(entityId: number): Promise<void> {
    if (isNaN(entityId)) entityId = 0;
    currentEntity.value = entityId;
    locationtree.value = buildLocationTree(entityId);
    import("../router").then(({ default: router }) => {
      router.push({ query: entityId === 0 ? {} : { entity: entityId } });
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
      apiSearchResults.value = results.map((r) => r.entity);
      // Store image scores for display on cards
      for (const r of results) {
        apiSearchScores.value[r.entity.id] = { image: r.imageScore };
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

  function readname(entityId: number): string {
    if (entityId === 0) return "World";
    return nameMap.value[entityId] ?? String(entityId);
  }

  function hasChildren(entityId: number): boolean {
    return allRecords.value.some((r) => (r.ParentID ?? 0) === entityId);
  }

  function listChildLocations(entityId: number): number[] {
    return allRecords.value
      .filter((r) => (r.ParentID ?? 0) === entityId)
      .map((r) => r.ID)
      .sort((a, b) => {
        const na = nameMap.value[a]?.toLowerCase() ?? "";
        const nb = nameMap.value[b]?.toLowerCase() ?? "";
        return na.localeCompare(nb, undefined, { numeric: true });
      });
  }

  function load(locationId: number, searchTextVal: string): Entity[] {
    if (searchTextVal.trim()) {
      let results = [...apiSearchResults.value];
      if (filterToMissingImage.value) {
        results = results.filter(
          (e) => !e.artifacts || e.artifacts.length === 0,
        );
      } else if (filterToOnlyImage.value) {
        results = results.filter((e) => e.artifacts && e.artifacts.length > 0);
      }
      return results;
    }
    return allRecords.value
      .filter((r) => (r.ParentID ?? 0) === locationId)
      .map(recordToEntity)
      .sort((a, b) => {
        const na = (a.name ?? "").toLowerCase();
        const nb = (b.name ?? "").toLowerCase();
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
      currentEntity,
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
            !filterworld.value && currentEntity.value !== 0
              ? currentEntity.value
              : undefined;
          const { results, partial } = await api.searchRecords(query, {
            parentId: scopeId,
            searchImage: searchImage.value,
            searchTextEmbedded: searchTextEmbedded.value,
            searchTextSubstring: searchTextSubstring.value,
          });
          if (partial) {
            const scope: PartialScope = {
              scopeId,
              searchImage: searchImage.value,
              searchTextEmbedded: searchTextEmbedded.value,
            };
            partialScope = scope;
            progressToastId = useToastsStore().add(
              "Indexing embeddings — results may be incomplete",
              "warn",
              true,
            );

            fetchAndUpdateProgress(scope); // populates searchProgress
          } else {
            partialScope = null;
          }
          apiSearchResults.value = results.map((r) => r.entity);
          const scores: Record<number, { image?: number; text?: number }> = {};
          for (const r of results) {
            scores[r.entity.id] = { image: r.imageScore, text: r.textScore };
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

  // Clear progress toast on navigation
  watch(currentEntity, () => clearProgressToast());

  // Update progress toast based on searchProgress state
  watch(
    () => searchProgress.value,
    (progress) => {
      if (!progress) {
        DEBUG && console.log("[entities] progress watch: progress is falsy");
        return;
      }
      const record = progress.record;
      const artifact = progress.artifact;
      // Skip if toast already finalized - prevents redundant watch fires
      if (progressToastId === null) {
        DEBUG &&
          console.log("[entities] progress watch: toast already finalized");
        return;
      }
      if (!record || !artifact) {
        DEBUG &&
          console.log("[entities] progress watch: missing record/artifact");
        return;
      }
      DEBUG &&
        console.log(
          "[entities] embedding progress total:",
          progress.record?.complete?.length ?? 0,
          progress.record?.pending?.length ?? 0,
          progress.artifact?.complete?.length ?? 0,
          progress.artifact?.pending?.length ?? 0,
          "progressToastId:",
          progressToastId,
        );
      const total =
        (record?.complete?.length || 0) +
        (record?.pending?.length || 0) +
        (artifact?.complete?.length || 0) +
        (artifact?.pending?.length || 0);
      const currentCount =
        (record?.pending?.length || 0) + (artifact?.pending?.length || 0);
      if (currentCount === 0) {
        if (progressToastId !== null) {
          DEBUG &&
            console.log("[entities] embedding complete, finalizing toast");
          useToastsStore().update(progressToastId, "Embeddings ready", "success");
          useToastsStore().finalize(progressToastId);
          progressToastId = null;
          partialScope = null;
        }
        return;
      }
      if (progressToastId === null) {
        DEBUG && console.log("[entities] progressToastId null, creating toast");
        progressToastId = useToastsStore().add(
          "Indexing embeddings...",
          "warn",
          true,
        );
      } else {
        DEBUG &&
          console.log(
            "[entities] progressToastId exists:",
            progressToastId,
            "current:",
            currentCount,
            "total:",
            total,
          );
      }
      const newMessage = `Indexed ${total - currentCount}/${total} embeddings`;
      DEBUG && console.log("[entities] toast update:", newMessage);
      useToastsStore().update(progressToastId, newMessage);
    },
    { deep: true },
  );

  return {
    currentEntity,
    allRecords,
    nameMap,
    entityMap,
    locationtree,
    searchtextpredebounce,
    searchtext,
    moveSearchtext,
    selectedEntityId,
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
    setCurrentEntity,
    searchByImage,
    readname,
    hasChildren,
    listChildLocations,
    load,
    debouncesearch,
  };
});
