<script setup lang="ts">
import { onMounted, onUnmounted, watch, ref, computed, nextTick } from "vue";
import { useRecordsStore } from "@/stores/records";
import { useCameraStore } from "@/stores/camera";
import { useToast } from "primevue/usetoast";
import { useAuthStore } from "@/stores/auth";
import { DEFAULT_TOAST_LIFE } from "@/stores/constants";
import RecordCard from "@/components/RecordCard.vue";
import CameraModal from "@/components/CameraModal.vue";
import NewRecordDialog from "@/components/NewRecordDialog.vue";
import CommandDialog from "@/components/CommandDialog.vue";
import SearchBar from "@/components/SearchBar.vue";
import BreadcrumbNav from "@/components/BreadcrumbNav.vue";
import KbdHint from "@/components/KbdHint.vue";
import PlusIcon from "vue-material-design-icons/Plus.vue";
import CameraIcon from "vue-material-design-icons/Camera.vue";
import LogoutIcon from "vue-material-design-icons/Logout.vue";
import ImageSearchIcon from "vue-material-design-icons/ImageSearch.vue";
import { api } from "@/api";
import { useRoute } from "vue-router";

const route = useRoute();
const recordsStore = useRecordsStore();
const cameraStore = useCameraStore();
const toast = useToast();
const authStore = useAuthStore();

const newRecordVisible = ref(false);
const newRecordLocation = ref(0);
const confirmMoveId = ref<number | null>(null);
const commandDialogVisible = ref(false);
const selectedRecordId = ref<number | null>(null);
const showHint = ref(false);
const editRecordId = ref<number | null>(null);
const cardRefs = ref<Record<number, { cardEl: HTMLElement | null }>>({});
const deleteConfirmId = ref<number | null>(null);
const searchBarRef = ref<{ focusSearch: () => void } | null>(null);
const editingCardId = ref<number | null>(null);

const handleLogout = (): void => {
  authStore.clearToken();
  window.location.href = "/";
};

const visibleRecords = computed(() =>
  recordsStore.load(recordsStore.currentRecord, recordsStore.searchtext),
);

const anyDialogOpen = computed(
  () =>
    newRecordVisible.value ||
    confirmMoveId.value !== null ||
    commandDialogVisible.value,
);

const handleMoveConfirmed = async (
  recordId: number,
  newLocation: number,
): Promise<void> => {
  const idx = visibleRecords.value.findIndex((e) => e.ID === recordId);
  const rest = visibleRecords.value.filter((e) => e.ID !== recordId);
  const nextId =
    rest.length > 0 ? rest[Math.min(idx, rest.length - 1)]!.ID : null;
  confirmMoveId.value = null;
  selectedRecordId.value = null;
  try {
    await api.moveRecord(recordId, newLocation);
    await recordsStore.reload();
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Record Moved",
      detail: "Record moved successfully",
      life: DEFAULT_TOAST_LIFE,
    });
    if (newLocation === recordsStore.currentRecord) {
      selectedRecordId.value = recordId;
    } else if (nextId !== null) {
      selectedRecordId.value = nextId;
    }
  } catch {
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Failed to Move Record",
      detail: "Failed to move record",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const handleFabCapture = async (): Promise<void> => {
  const capturedFiles: File[] = [];
  await new Promise<void>((resolve) => {
    cameraStore.open((files: File[]) => {
      capturedFiles.push(...files);
      resolve();
    });
  });
  if (!capturedFiles[0]) return;
  try {
    const artifactId = await api.uploadArtifact(capturedFiles[0]);
    await api.createRecord({
      ParentID: recordsStore.currentRecord || undefined,
      Artifacts: [artifactId],
    });
    await recordsStore.reload();
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Record Created",
      detail: "Record created from photo",
      life: DEFAULT_TOAST_LIFE,
    });
  } catch {
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Failed to Create Record",
      detail: "Failed to create record from photo",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const handleFabImageSearch = async (): Promise<void> => {
  const capturedFiles: File[] = [];
  await new Promise<void>((resolve) => {
    cameraStore.open((files: File[]) => {
      capturedFiles.push(...files);
      resolve();
    });
  });
  if (!capturedFiles[0]) return;
  try {
    await recordsStore.searchByImage(capturedFiles[0]);
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Image Search Complete",
      detail: "Image search complete",
      life: DEFAULT_TOAST_LIFE,
    });
  } catch {
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Failed to Search",
      detail: "Failed to search for similar records",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const confirmDeleteRecord = async (recordId: number): Promise<void> => {
  const beforeList = visibleRecords.value.filter((e) => e.ID !== recordId);
  const idx = visibleRecords.value.findIndex((e) => e.ID === recordId);
  const nextId =
    beforeList.length > 0
      ? beforeList[Math.min(idx, beforeList.length - 1)]!.ID
      : null;
  deleteConfirmId.value = null;
  selectedRecordId.value = null;
  try {
    await api.deleteRecord(recordId);
    await recordsStore.reload();
    toast.add({
      severity: "warn",
      summary: "Record Deleted",
      detail: "Record deleted successfully",
      life: DEFAULT_TOAST_LIFE,
    });
    if (nextId !== null) {
      selectedRecordId.value = nextId;
    }
  } catch {
    toast.add({
      severity: "error",
      summary: "Failed to Delete Record",
      detail: "Failed to delete record",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const handleQuickCaptureOnRecord = async (recordId: number): Promise<void> => {
  const capturedFiles: File[] = [];
  await new Promise<void>((resolve) => {
    cameraStore.open((files: File[]) => {
      capturedFiles.push(...files);
      resolve();
    });
  });
  if (!capturedFiles[0]) return;
  try {
    const artifactId = await api.uploadArtifact(capturedFiles[0]);
    const appRecord = recordsStore.recordMap[recordId];
    const artifacts = [
      ...(appRecord?.Artifacts ?? []).map((a) => a.ID),
      artifactId,
    ];
    await api.patchRecord(recordId, { Artifacts: artifacts });
    await recordsStore.reload();
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Artifact Captured",
      detail: "Artifact captured and added",
      life: DEFAULT_TOAST_LIFE,
    });
  } catch {
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Failed to Capture Artifact",
      detail: "Failed to capture artifact",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const handleQuickCaptureNewChild = async (parentId: number): Promise<void> => {
  const capturedFiles: File[] = [];
  await new Promise<void>((resolve) => {
    cameraStore.open((files: File[]) => {
      capturedFiles.push(...files);
      resolve();
    });
  });
  if (!capturedFiles[0]) return;
  try {
    const artifactId = await api.uploadArtifact(capturedFiles[0]);
    await api.createRecord({
      ParentID: parentId || undefined,
      Artifacts: [artifactId],
    });
    await recordsStore.reload();
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Record Created",
      detail: "Record created from photo",
      life: DEFAULT_TOAST_LIFE,
    });
  } catch {
    const toast = useToast();
    toast.add({
      severity: "error",
      summary: "Failed to Create Record",
      detail: "Failed to create record from photo",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};

const navigateGrid = (direction: "up" | "down" | "left" | "right"): void => {
  const records = visibleRecords.value;
  if (records.length === 0) return;

  if (selectedRecordId.value === null) {
    selectedRecordId.value = records[0]!.ID;
    return;
  }

  const currentEl = cardRefs.value[selectedRecordId.value]?.cardEl;
  if (!currentEl) return;

  const cur = currentEl.getBoundingClientRect();
  const curCX = cur.left + cur.width / 2;
  const curCY = cur.top + cur.height / 2;

  let bestId: number | null = null;
  let bestScore = Infinity;

  for (const rec of records) {
    if (rec.ID === selectedRecordId.value) continue;
    const el = cardRefs.value[rec.ID]?.cardEl;
    if (!el) continue;
    const r = el.getBoundingClientRect();
    const cx = r.left + r.width / 2;
    const cy = r.top + r.height / 2;
    const dx = cx - curCX;
    const dy = cy - curCY;

    const inDir =
      direction === "right"
        ? dx > 10
        : direction === "left"
          ? dx < -10
          : direction === "down"
            ? dy > 10
            : dy < -10;
    if (!inDir) continue;

    const primary =
      direction === "left" || direction === "right"
        ? Math.abs(dx)
        : Math.abs(dy);
    const secondary =
      direction === "left" || direction === "right"
        ? Math.abs(dy)
        : Math.abs(dx);
    const score = primary + secondary * 3;
    if (score < bestScore) {
      bestScore = score;
      bestId = rec.ID;
    }
  }

  if (bestId !== null) selectedRecordId.value = bestId;
};

const handleKeydown = (e: KeyboardEvent): void => {
  if (e.key === "Meta" || e.key === "Alt") {
    showHint.value = true;
    return;
  }

  const tag = (e.target as HTMLElement)?.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;

  if (e.key === "Escape") {
    commandDialogVisible.value = false;
    deleteConfirmId.value = null;
    confirmMoveId.value = null;
    selectedRecordId.value = null;
    return;
  }

  if (anyDialogOpen.value) return;

  switch (e.key) {
    case "/":
      e.preventDefault();
      searchBarRef.value?.focusSearch();
      break;

    case "?":
      e.preventDefault();
      commandDialogVisible.value = true;
      break;

    case "g":
    case "G":
      e.preventDefault();
      recordsStore.filterworld = !recordsStore.filterworld;
      break;

    case "i":
    case "I":
      if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        recordsStore.searchImage = !recordsStore.searchImage;
      }
      break;

    case "w":
    case "W":
      if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        recordsStore.searchTextEmbedded = !recordsStore.searchTextEmbedded;
      }
      break;

    case "t":
    case "T":
      if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        recordsStore.searchTextSubstring = !recordsStore.searchTextSubstring;
      }
      break;

    case "ArrowDown":
      e.preventDefault();
      navigateGrid("down");
      break;
    case "ArrowUp":
      e.preventDefault();
      navigateGrid("up");
      break;
    case "ArrowRight":
      e.preventDefault();
      navigateGrid("right");
      break;
    case "ArrowLeft":
      e.preventDefault();
      navigateGrid("left");
      break;

    case "Enter":
      if (cameraStore.opened || editingCardId.value !== null) break;
      e.preventDefault();
      if (deleteConfirmId.value !== null) {
        confirmDeleteRecord(deleteConfirmId.value);
      } else if (selectedRecordId.value !== null) {
        recordsStore.setCurrentRecord(selectedRecordId.value).then(() => {
          nextTick(() => {
            if (visibleRecords.value.length > 0) {
              selectedRecordId.value = visibleRecords.value[0]!.ID;
            }
          });
        });
      }
      break;

    case "Backspace":
      e.preventDefault();
      {
        const cur = recordsStore.currentRecord;
        if (cur === 0) break;
        const prevId = cur;
        const tree = recordsStore.locationtree;
        const parentId = tree.length >= 2 ? tree[tree.length - 2]! : 0;
        recordsStore.setCurrentRecord(parentId).then(() => {
          nextTick(() => {
            selectedRecordId.value = prevId;
          });
        });
      }
      break;

    case "Delete":
    case "d":
    case "D":
      if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        if (deleteConfirmId.value !== null) {
          e.preventDefault();
          confirmDeleteRecord(deleteConfirmId.value);
        } else if (selectedRecordId.value !== null) {
          e.preventDefault();
          deleteConfirmId.value = selectedRecordId.value;
        }
      }
      break;

    case "e":
    case "E":
      if (
        !e.shiftKey &&
        !e.metaKey &&
        !e.ctrlKey &&
        selectedRecordId.value !== null
      ) {
        e.preventDefault();
        editRecordId.value = selectedRecordId.value;
      }
      break;

    case "p":
    case "P":
      if (
        !e.shiftKey &&
        !e.metaKey &&
        !e.ctrlKey &&
        selectedRecordId.value !== null
      ) {
        e.preventDefault();
        handleQuickCaptureOnRecord(selectedRecordId.value);
      }
      break;

    case "c":
    case "C":
      if (
        e.shiftKey &&
        !e.metaKey &&
        !e.ctrlKey &&
        selectedRecordId.value !== null
      ) {
        e.preventDefault();
        handleQuickCaptureNewChild(selectedRecordId.value);
      } else if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        handleFabCapture();
      }
      break;

    case "n":
    case "N":
      if (
        e.shiftKey &&
        !e.metaKey &&
        !e.ctrlKey &&
        selectedRecordId.value !== null
      ) {
        e.preventDefault();
        newRecordLocation.value = selectedRecordId.value;
        newRecordVisible.value = true;
      } else if (!e.shiftKey && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        newRecordLocation.value = recordsStore.currentRecord;
        newRecordVisible.value = true;
      }
      break;

    case "m":
    case "M":
      if (
        !e.shiftKey &&
        !e.metaKey &&
        !e.ctrlKey &&
        selectedRecordId.value !== null
      ) {
        e.preventDefault();
        confirmMoveId.value = selectedRecordId.value;
      }
      break;
  }
};

const handleKeyup = (e: KeyboardEvent): void => {
  if (e.key === "Meta" || e.key === "Alt") {
    showHint.value = false;
  }
};

onMounted(() => {
  window.addEventListener("keydown", handleKeydown);
  window.addEventListener("keyup", handleKeyup);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleKeydown);
  window.removeEventListener("keyup", handleKeyup);
});

watch(selectedRecordId, (newId) => {
  if (deleteConfirmId.value !== null && newId !== deleteConfirmId.value) {
    deleteConfirmId.value = null;
  }
});

watch(
  () => recordsStore.currentRecord,
  () => {
    selectedRecordId.value = null;
    deleteConfirmId.value = null;
  },
);

watch(
  () => route.query.record,
  async (newId) => {
    const id = parseInt(newId as string, 10);
    if (!isNaN(id)) {
      await recordsStore.setCurrentRecord(id);
    }
  },
);
</script>

<template>
  <div
    class="min-h-screen bg-gray-50 text-gray-900 dark:bg-gray-900 dark:text-white"
  >
    <div
      v-if="recordsStore.isLoading && recordsStore.allRecords.length === 0"
      class="flex h-screen items-center justify-center"
    >
      <span class="text-2xl text-gray-500">Loading...</span>
    </div>

    <div v-else>
      <div class="w-full px-4 pt-4 pb-4">
        <div class="flex items-center gap-2">
          <BreadcrumbNav
            @open-new-record="
              newRecordLocation = recordsStore.currentRecord;
              newRecordVisible = true;
            "
          />
          <router-link
            to="/settings"
            class="flex shrink-0 items-center gap-1 rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
            title="Settings"
          >
            Settings
          </router-link>
          <PrimeVueButton
            v-if="authStore.isAuthenticated"
            @click="handleLogout"
            severity="secondary"
            title="Logout"
            class="min-w-max gap-2"
          >
            <span class="truncate text-sm font-medium">Logout</span>
            <LogoutIcon :size="18" />
          </PrimeVueButton>
        </div>
        <SearchBar ref="searchBarRef" :show-hint="showHint" />
      </div>

      <div class="mt-8 w-full px-4">
        <div
          v-if="recordsStore.searching"
          class="flex h-64 flex-col items-center justify-center gap-4"
        >
          <div
            class="h-12 w-12 animate-spin rounded-full border-b-2 border-blue-500"
          ></div>
          <p class="text-xl text-gray-500/50">Searching...</p>
        </div>
        <div
          v-else-if="visibleRecords.length === 0"
          class="flex h-64 items-center justify-center"
        >
          <p class="text-2xl text-gray-500/50">Empty</p>
        </div>

        <div class="flex flex-wrap justify-center gap-4">
          <TransitionGroup name="fade">
            <RecordCard
              v-for="rec in visibleRecords"
              :key="rec.ID"
              :ref="
                (el: any) => {
                  if (el) cardRefs[rec.ID] = el;
                  else delete cardRefs[rec.ID];
                }
              "
              :app-record="rec"
              :is-selected="selectedRecordId === rec.ID"
              :show-hint="showHint"
              :start-edit="editRecordId === rec.ID"
              :confirm-delete="deleteConfirmId === rec.ID"
              :confirm-move="confirmMoveId === rec.ID"
              @select="
                selectedRecordId = rec.ID;
                deleteConfirmId = null;
              "
              @create-child="
                (id) => {
                  newRecordLocation = id;
                  newRecordVisible = true;
                }
              "
              @request-move="
                (id) => {
                  confirmMoveId = id;
                }
              "
              @edit-started="
                editRecordId = null;
                editingCardId = rec.ID;
              "
              @edit-ended="editingCardId = null"
              @request-delete="
                selectedRecordId = rec.ID;
                deleteConfirmId = rec.ID;
              "
              @delete-confirmed="confirmDeleteRecord(rec.ID)"
              @delete-cancelled="deleteConfirmId = null"
              @move-confirmed="
                (newLocation) => handleMoveConfirmed(rec.ID, newLocation)
              "
              @move-cancelled="confirmMoveId = null"
            />
          </TransitionGroup>
        </div>
      </div>
    </div>

    <div class="fixed right-6 bottom-6 flex flex-col gap-3">
      <PrimeVueButton
        @click="
          newRecordLocation = recordsStore.currentRecord;
          newRecordVisible = true;
        "
        rounded
        class="relative h-14 w-14 p-0"
        title="Create new record (N)"
      >
        <PlusIcon :size="28" />
        <KbdHint contents="N" :show="showHint" />
      </PrimeVueButton>
      <PrimeVueButton
        @click="handleFabCapture"
        rounded
        class="relative h-14 w-14 p-0"
        title="Quick capture (C)"
      >
        <CameraIcon :size="28" />
        <KbdHint contents="C" :show="showHint" />
      </PrimeVueButton>
      <PrimeVueButton
        @click="handleFabImageSearch"
        rounded
        severity="help"
        class="relative h-14 w-14 p-0"
        title="Image search (I)"
      >
        <ImageSearchIcon :size="28" />
        <KbdHint contents="I" :show="showHint" />
      </PrimeVueButton>
    </div>

    <CameraModal />

    <NewRecordDialog
      :visible="newRecordVisible"
      :location="newRecordLocation"
      :show-hint="showHint"
      @update:visible="newRecordVisible = $event"
      @created="
        (id) => {
          if (newRecordLocation === recordsStore.currentRecord)
            selectedRecordId = id;
        }
      "
    />
    <CommandDialog
      :visible="commandDialogVisible"
      @update:visible="commandDialogVisible = $event"
    />
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
