<script setup lang="ts" name="RecordCard">
import { ref, computed, watch, nextTick, onUnmounted } from "vue";
import { useRecordsStore } from "@/stores/records";
import { useCameraStore } from "@/stores/camera";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";
import type { BackendRecord } from "@/api/types";
import KbdHint from "@/components/KbdHint.vue";
import ArtifactImage from "@/components/ArtifactImage.vue";
import TrashCanIcon from "vue-material-design-icons/TrashCan.vue";
import FolderMoveIcon from "vue-material-design-icons/FolderMove.vue";
import PencilIcon from "vue-material-design-icons/Pencil.vue";
import CameraIcon from "vue-material-design-icons/Camera.vue";
import CameraPlusIcon from "vue-material-design-icons/CameraPlus.vue";
import PlusIcon from "vue-material-design-icons/Plus.vue";
import CheckIcon from "vue-material-design-icons/Check.vue";
import CloseIcon from "vue-material-design-icons/Close.vue";
import ArrowUpIcon from "vue-material-design-icons/ArrowUp.vue";
import AlertIcon from "vue-material-design-icons/Alert.vue";
import ImageSearchIcon from "vue-material-design-icons/ImageSearch.vue";

const props = defineProps<{
  appRecord: BackendRecord;
  isSelected?: boolean;
  showHint?: boolean;
  startEdit?: boolean;
  confirmDelete?: boolean;
  confirmMove?: boolean;
}>();

const emit = defineEmits<{
  recordUpdated: [appRecord: BackendRecord];
  createChild: [locationId: number];
  requestMove: [recordId: number];
  select: [];
  editStarted: [];
  editEnded: [];
  deleteConfirmed: [];
  deleteCancelled: [];
  requestDelete: [];
  moveConfirmed: [newLocation: number];
  moveCancelled: [];
}>();

const recordsStore = useRecordsStore();
const cameraStore = useCameraStore();
const toastsStore = useToastsStore();

const cardEl = ref<HTMLElement | null>(null);
const nameInputEl = ref<HTMLInputElement | null>(null);
const editMode = ref(false);
const localRecord = ref<BackendRecord>({
  ...props.appRecord,
});
const pendingDeletions = ref<Set<number>>(new Set());

const isDragOver = ref(false);
const isDragging = ref(false);
const pointerOnEditable = ref(false);

const childDragReadyId = ref<number | null>(null);
let childDragTimer: ReturnType<typeof setTimeout> | null = null;
const draggingChildId = ref<number | null>(null);
const isDragOverChildren = ref(false);

const handleChildDragStart = (e: DragEvent, childId: number): void => {
  e.stopPropagation();
  e.dataTransfer?.setData("recordId", childId.toString());
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
  draggingChildId.value = childId;
};

const handleChildDragEnd = (): void => {
  draggingChildId.value = null;
};

const handleChildDragOver = (e: DragEvent, childId: number): void => {
  e.preventDefault();
  e.stopPropagation();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  if (childDragReadyId.value === childId) return;
  if (childDragTimer !== null) return;
  childDragTimer = setTimeout(() => {
    childDragReadyId.value = childId;
    childDragTimer = null;
  }, 1000);
};

const handleChildDragLeave = (e: DragEvent): void => {
  if ((e.currentTarget as HTMLElement).contains(e.relatedTarget as Node))
    return;
  if (childDragTimer !== null) {
    clearTimeout(childDragTimer);
    childDragTimer = null;
  }
  childDragReadyId.value = null;
};

const handleChildDrop = async (
  e: DragEvent,
  childId: number,
): Promise<void> => {
  e.stopPropagation();
  if (childDragTimer !== null) {
    clearTimeout(childDragTimer);
    childDragTimer = null;
  }
  childDragReadyId.value = null;
  if (!e.dataTransfer?.getData("recordId")) return;
  const recordId = parseInt(e.dataTransfer.getData("recordId"), 10);
  if (isNaN(recordId) || recordId === childId) return;
  try {
    await api.moveRecord(recordId, childId);
    await recordsStore.reload();
    toastsStore.add("Record moved", "info");
  } catch {
    toastsStore.add("Failed to move record");
  }
};
const isDraggable = computed(
  () =>
    !pointerOnEditable.value &&
    !editMode.value &&
    !props.confirmDelete &&
    !props.confirmMove,
);

const handlePointerDown = (e: PointerEvent): void => {
  pointerOnEditable.value = !!(e.target as HTMLElement).closest(
    "input, textarea, [contenteditable]",
  );
};

const handlePointerUp = (): void => {
  pointerOnEditable.value = false;
};

const handleDragStart = (e: DragEvent): void => {
  const el = cardEl.value;
  if (!el) return;
  e.dataTransfer?.setData("recordId", props.appRecord.ID.toString());
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
  isDragging.value = true;
};

const handleDragEnd = (): void => {
  isDragging.value = false;
};

const handleDragOver = (e: DragEvent): void => {
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  isDragOver.value = true;
};

const handleDragLeave = (e: DragEvent): void => {
  if (!(e.currentTarget as HTMLElement)?.contains(e.relatedTarget as Node)) {
    isDragOver.value = false;
    isDragOverChildren.value = false;
  }
};

const handleDrop = async (e: DragEvent): Promise<void> => {
  e.preventDefault();
  isDragOver.value = false;
  const draggedId = parseInt(e.dataTransfer?.getData("recordId") ?? "");
  if (isNaN(draggedId) || draggedId === props.appRecord.ID) return;
  if (isDescendantOf(props.appRecord.ID, draggedId)) return;
  // Child dragged back over its own children box — leave it where it is
  if (draggingChildId.value !== null && isDragOverChildren.value) return;
  // Child being dragged out → move to this card's parent level, not into the card
  const targetId =
    draggingChildId.value !== null
      ? props.appRecord.ParentID
      : props.appRecord.ID;
  try {
    await api.moveRecord(draggedId, targetId);
    await recordsStore.reload();
    toastsStore.add("Record moved", "info");
  } catch {
    toastsStore.add("Failed to move record");
  }
};

const moveTargetLocation = ref<number>(0);
const moveSearchInputRef = ref<HTMLInputElement | null>(null);
const nextRefPlaceholder = ref<string | null>(null);

const nameIsWrongNumber = computed(() => {
  const n = localRecord.value.Title;
  return !!n && /^\d+$/.test(n) && parseInt(n, 10) !== props.appRecord.ID;
});

const nameRefMismatch = computed(() => {
  const n = localRecord.value.Title;
  const r = localRecord.value.ReferenceNumber;
  return !!n && !!r && /^\d+$/.test(n) && /^\d+$/.test(r) && n !== r;
});

const refTaken = computed(() => {
  const v = localRecord.value.ReferenceNumber?.trim();
  if (!v) return false;
  return Object.values(recordsStore.recordMap).some(
    (e) => e.ID !== props.appRecord.ID && e.ReferenceNumber === v,
  );
});

watch(editMode, async (on) => {
  if (on) {
    nextRefPlaceholder.value = String(
      await api.nextReferenceNumber([localRecord.value.ID]),
    );
  } else {
    nextRefPlaceholder.value = null;
  }
});

const isDescendantOf = (recordId: number, ancestorId: number): boolean => {
  let current = recordId;
  while (current !== 0) {
    if (current === ancestorId) return true;
    const parent = recordsStore.recordMap[current];
    if (!parent) break;
    current = parent.ParentID ?? 0;
  }
  return false;
};

const moveUp = (): void => {
  if (recordsStore.currentRecord !== 0) {
    const currentRec = recordsStore.recordMap[recordsStore.currentRecord];
    if (currentRec?.ParentID !== undefined) {
      emit("moveConfirmed", currentRec.ParentID);
    }
  }
};

const filteredMoveRecords = computed(() => {
  const term = recordsStore.moveSearchtext.toLowerCase().trim();
  const world: BackendRecord = {
    ID: 0,
    Title: "World",
    Description: "",
    Quantity: null,
    Artifacts: [],
    ParentID: 0,
    ReferenceNumber: null,
    lastModified: null,
  };
  const candidates: BackendRecord[] = [
    ...Object.values(recordsStore.recordMap).filter(
      (e) =>
        e.ID !== props.appRecord.ID &&
        !isDescendantOf(e.ID, props.appRecord.ID),
    ),
    world,
  ];
  if (!term) return candidates;
  return candidates.filter(
    (e) =>
      e.Title?.toLowerCase().includes(term) ||
      e.Description?.toLowerCase().includes(term) ||
      e.ID.toString().includes(term) ||
      e.ReferenceNumber?.toString().includes(term),
  );
});

const currentLocationName = computed(() => {
  if (recordsStore.currentRecord === 0) return "World";
  return recordsStore.readname(recordsStore.currentRecord);
});

const isAtCurrentLocation = computed((): boolean => {
  if (props.appRecord.ParentID === undefined) return false;
  return props.appRecord.ParentID === recordsStore.currentRecord;
});

const handleMoveKeydown = (e: KeyboardEvent): void => {
  if (!props.confirmMove) return;
  const target = e.target as HTMLElement;

  // If focused on input/select, only handle Enter/Escape
  if (target.matches("input, select")) {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopImmediatePropagation();
      (target as HTMLElement).blur();
      return;
    } else if (e.key === "Enter") {
      e.preventDefault();
      e.stopImmediatePropagation();
      emit("moveConfirmed", moveTargetLocation.value);
      return;
    }
    // Block H and U shortcuts when typing in input
    if (e.key === "h" || e.key === "H" || e.key === "u" || e.key === "U") {
      return;
    }
  }

  // Handle H/U shortcuts only when not in input
  if (e.key === "h" || e.key === "H") {
    e.preventDefault();
    e.stopImmediatePropagation();
    emit("moveConfirmed", recordsStore.currentRecord);
  } else if (e.key === "u" || e.key === "U") {
    e.preventDefault();
    e.stopImmediatePropagation();
    moveUp();
  }
};

watch(
  () => props.confirmMove,
  (val) => {
    if (val) {
      const results = filteredMoveRecords.value;
      const hasSearch = recordsStore.moveSearchtext.trim() !== "";
      moveTargetLocation.value =
        hasSearch && results.length > 0
          ? (results[0]?.ID ?? 0)
          : recordsStore.currentRecord;
      window.addEventListener("keydown", handleMoveKeydown, true);
      nextTick(() => moveSearchInputRef.value?.focus());
    } else {
      window.removeEventListener("keydown", handleMoveKeydown, true);
    }
  },
);

watch(filteredMoveRecords, (results) => {
  if (!props.confirmMove) return;
  if (results.some((r) => r.ID === moveTargetLocation.value)) return;
  const term = recordsStore.moveSearchtext.toLowerCase().trim();
  const byRef =
    term && results.find((r) => r.ReferenceNumber?.toLowerCase() === term);
  const byId = term && results.find((r) => r.ID.toString() === term);
  moveTargetLocation.value = (byRef || byId || results[0])?.ID ?? 0;
});

const formatOptionSegments = (
  recordId: number,
): { text: string; isRef: boolean }[] => {
  const tree: { text: string; isRef: boolean }[] = [];
  let target = recordId;
  while (target !== 0) {
    const elem = recordsStore.recordMap[target];
    if (!elem) {
      tree.push({ text: target.toString(), isRef: false });
      break;
    }
    if (elem.Title) {
      tree.push({ text: elem.Title, isRef: false });
    } else if (elem.ReferenceNumber) {
      tree.push({
        text: `#${elem.ReferenceNumber}`,
        isRef: true,
      });
    } else {
      tree.push({ text: target.toString(), isRef: false });
    }
    target = elem.ParentID ?? 0;
  }
  tree.push({ text: "World", isRef: false });
  tree.reverse();
  return tree;
};

watch(
  () => props.isSelected,
  (val) => {
    if (val)
      nextTick(() =>
        (cardEl.value as HTMLElement)?.scrollIntoView({
          behavior: "smooth",
          block: "nearest",
        }),
      );
  },
);

watch(
  () => props.startEdit,
  (val) => {
    if (val && !editMode.value) {
      handleEditToggle();
      emit("editStarted");
    }
  },
);

const handleEditKeydown = (e: KeyboardEvent): void => {
  if (cameraStore.opened) return;
  const target = e.target as HTMLElement;
  if (e.key === "Escape") {
    e.preventDefault();
    e.stopImmediatePropagation();
    if (target.matches("input, textarea")) {
      (target as HTMLElement).blur();
    } else {
      handleCancel();
    }
  } else if (e.key === "Enter" && !target.matches("textarea")) {
    e.preventDefault();
    e.stopImmediatePropagation();
    handleSave();
  } else if (
    (e.key === "p" || e.key === "P") &&
    !target.matches("input, textarea")
  ) {
    e.preventDefault();
    e.stopImmediatePropagation();
    cameraStore.open((files: File[]) =>
      files.forEach((f) => handleEditArtifact(f)),
    );
  }
};

watch(editMode, (val) => {
  if (val) {
    window.addEventListener("keydown", handleEditKeydown, true);
    nextTick(() => nameInputEl.value?.focus());
  } else {
    window.removeEventListener("keydown", handleEditKeydown, true);
    emit("editEnded");
  }
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleEditKeydown, true);
  window.removeEventListener("keydown", handleMoveKeydown, true);
});

const handleUpdate = async (): Promise<void> => {
  const e = localRecord.value;
  try {
    await Promise.all(
      [...pendingDeletions.value].map((id) => api.deleteArtifact(id)),
    );
    const artifacts = (localRecord.value.Artifacts ?? []).filter(
      (id) => !pendingDeletions.value.has(id),
    );
    await api.updateRecord(props.appRecord.ID, {
      Title: e.Title || null,
      ReferenceNumber: e.ReferenceNumber || null,
      Description: e.Description,
      Quantity: typeof e.Quantity === "number" ? e.Quantity : null,
      ParentID: e.ParentID || undefined,
      Artifacts: artifacts,
    });
    pendingDeletions.value = new Set();
    await recordsStore.reload();
    editMode.value = false;
    emit("recordUpdated", localRecord.value);
    toastsStore.add("Record updated", "info");
  } catch (error) {
    console.error("Failed to update record:", error);
    toastsStore.add("Failed to update record");
  }
};

const handleDelete = async (): Promise<void> => {
  try {
    await api.deleteRecord(props.appRecord.ID);
    await recordsStore.reload();
    toastsStore.add("Record deleted", "warn");
  } catch (error) {
    console.error("Failed to delete record:", error);
    toastsStore.add("Failed to delete record");
  }
};

const handleQuickCapture = async (): Promise<void> => {
  await new Promise<void>((resolve) => {
    cameraStore.open((files: File[]) => {
      handleQuickCaptureCallback(files);
      resolve();
    });
  });
};

const handleQuickCaptureCallback = async (files: File[]): Promise<void> => {
  if (files.length === 0 || !files[0]) return;
  try {
    const artifactId = await api.uploadArtifact(files[0]);
    const artifacts = [...(props.appRecord.Artifacts ?? []), artifactId];
    await api.patchRecord(props.appRecord.ID, { Artifacts: artifacts });
    await recordsStore.reload();
    editMode.value = false;
    emit("recordUpdated", props.appRecord);
    toastsStore.add("Artifact captured and added", "info");
  } catch (error) {
    console.error("Failed to capture artifact:", error);
    toastsStore.add("Failed to capture artifact");
  }
};

const handleSearchByImage = async (): Promise<void> => {
  if (!props.appRecord.Artifacts || props.appRecord.Artifacts.length === 0)
    return;

  try {
    const artifactId = props.appRecord.Artifacts![0];
    const response = await fetch(`/api/artifact/${artifactId}`);
    const artifactFile = await response.blob();
    const file = new File([artifactFile], `artifact-${artifactId}.jpg`, {
      type: "image/jpeg",
    });

    await recordsStore.searchByImage(file);

    if (recordsStore.apiSearchResults.length > 0) {
      const message = `Found ${recordsStore.apiSearchResults.length} similar record(s)`;
      toastsStore.add(message, "info");
      console.log("Similar records:", recordsStore.apiSearchResults);
      console.log("Partial:", recordsStore.apiSearchResultsPartial);
    } else {
      toastsStore.add("No similar records found", "info");
    }
  } catch (error) {
    console.error("Image search failed:", error);
    toastsStore.add("Failed to search for similar records");
  }
};

const handleQuickCaptureNewChild = async (): Promise<void> => {
  await new Promise<void>((resolve) => {
    cameraStore.open(async (files: File[]) => {
      if (!files[0]) {
        resolve();
        return;
      }
      try {
        const artifactId = await api.uploadArtifact(files[0]);
        await recordsStore.reload();
        await api.patchRecord(props.appRecord.ID, { Artifacts: [artifactId] });
        toastsStore.add("Record created from photo");
      } catch {
        toastsStore.add("Failed to create record from photo");
      }
      resolve();
    });
  });
};

const handleEditToggle = (): void => {
  if (!editMode.value) {
    localRecord.value = {
      ...props.appRecord,
    };
  }
  editMode.value = !editMode.value;
};

const handleSave = async (): Promise<void> => {
  await handleUpdate();
};

const handleCancel = (): void => {
  localRecord.value = {
    ...props.appRecord,
  };
  pendingDeletions.value = new Set();
  editMode.value = false;
};

const toggleArtifactDeletion = (artifactId: number): void => {
  const next = new Set(pendingDeletions.value);
  if (next.has(artifactId)) {
    next.delete(artifactId);
  } else {
    next.add(artifactId);
  }
  pendingDeletions.value = next;
};

const images = computed(() => {
  const artifacts = editMode.value
    ? localRecord.value.Artifacts
    : props.appRecord.Artifacts;
  return artifacts ?? [];
});

const handleEditArtifact = async (file: File): Promise<void> => {
  try {
    const artifactId = await api.uploadArtifact(file);
    const artifacts = [...(localRecord.value.Artifacts ?? []), artifactId];
    localRecord.value = { ...localRecord.value, Artifacts: artifacts };
    await api.updateRecord(props.appRecord.ID, { Artifacts: artifacts });
    await recordsStore.reload();
    emit("recordUpdated", localRecord.value);
    toastsStore.add("Artifact uploaded", "info");
  } catch (error) {
    console.error("Failed to upload artifact:", error);
    toastsStore.add("Failed to upload artifact");
  }
};

defineExpose({ cardEl });
</script>

<template>
  <figure
    ref="cardEl"
    :draggable="isDraggable"
    class="xs:w-full relative flex h-full min-h-48 w-sm cursor-default flex-col rounded-xl bg-white shadow-md transition-opacity dark:bg-gray-800"
    :class="[
      isSelected
        ? 'ring-2 shadow-blue-200 ring-blue-500 dark:shadow-blue-900'
        : isDragOver
          ? draggingChildId !== null && !isDragOverChildren
            ? 'bg-blue-50/50 ring-2 shadow-blue-100 ring-blue-400 dark:bg-blue-900/10 dark:shadow-blue-900/30'
            : childDragReadyId !== null
              ? 'bg-green-50/50 ring-2 ring-green-300 dark:bg-green-900/10 dark:ring-green-800'
              : 'bg-green-50 ring-2 shadow-green-200 ring-green-500 dark:bg-green-900/20 dark:shadow-green-900'
          : 'ring-1 ring-gray-500/25 hover:shadow-lg hover:ring-gray-500/50',
      isDragging ? 'opacity-40' : '',
    ]"
    @click="emit('select')"
    @pointerdown="handlePointerDown"
    @pointerup="handlePointerUp"
    @dragstart="handleDragStart"
    @dragend="handleDragEnd"
    @dragover="handleDragOver"
    @dragleave="handleDragLeave"
    @drop="handleDrop"
  >
    <!-- Delete confirmation overlay -->
    <div
      v-if="confirmDelete"
      class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 rounded-xl bg-white/90 backdrop-blur-sm dark:bg-gray-800/90"
      @click.stop
    >
      <p class="text-lg font-semibold text-red-600 dark:text-red-400">
        {{
          appRecord.Title || appRecord.ReferenceNumber
            ? `Delete "${appRecord.Title || `#${appRecord.ReferenceNumber}`}"?`
            : "Delete?"
        }}
      </p>
      <div class="flex gap-3">
        <button
          @click.stop="emit('deleteConfirmed')"
          class="relative h-9 rounded-full bg-red-500 px-4 text-sm text-white shadow hover:bg-red-600"
        >
          Delete
          <KbdHint contents="Enter" :show="showHint && isSelected" />
        </button>
        <button
          @click.stop="emit('deleteCancelled')"
          class="relative h-9 rounded-full bg-gray-200 px-4 text-sm shadow hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600"
        >
          Cancel
          <KbdHint contents="Esc" :show="showHint && isSelected" />
        </button>
      </div>
    </div>
    <!-- Move confirmation overlay -->
    <div
      v-if="confirmMove"
      class="absolute inset-0 z-10 flex flex-col gap-2 rounded-xl bg-white/95 p-4 backdrop-blur-sm dark:bg-gray-800/95"
      @click.stop
    >
      <p class="text-sm font-semibold text-gray-700 dark:text-gray-300">
        {{
          appRecord.Title || appRecord.ReferenceNumber
            ? `Move "${appRecord.Title || `#${appRecord.ReferenceNumber}`}" to:`
            : "Move to:"
        }}
      </p>
      <input
        ref="moveSearchInputRef"
        v-model="recordsStore.moveSearchtext"
        type="search"
        placeholder="Search locations..."
        class="w-full rounded-full bg-white px-3 py-1.5 text-sm ring-1 dark:bg-gray-900"
        @click.stop
      />
      <select
        v-model="moveTargetLocation"
        class="min-h-0 w-full flex-1 rounded-lg bg-white px-2 py-1 text-sm ring-1 dark:bg-gray-900"
        size="4"
        @click.stop
      >
        <option
          v-for="loc in filteredMoveRecords"
          :key="loc.ID"
          :value="loc.ID"
        >
          {{
            formatOptionSegments(loc.ID)
              .map((s) => s.text)
              .join("/")
          }}
        </option>
      </select>
      <div class="flex flex-wrap items-center gap-2">
        <button
          @click.stop="emit('moveConfirmed', moveTargetLocation)"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Move"
        >
          <CheckIcon :size="20" />
          <KbdHint contents="Enter" :show="showHint && isSelected" />
        </button>
        <button
          v-if="!isAtCurrentLocation"
          @click.stop="emit('moveConfirmed', recordsStore.currentRecord)"
          class="relative h-10 rounded-full bg-purple-500 px-3 text-sm text-white shadow hover:bg-purple-600"
        >
          To {{ currentLocationName }}
          <KbdHint contents="H" :show="showHint && isSelected" />
        </button>
        <button
          v-if="appRecord.ID !== 0 && recordsStore.currentRecord !== 0"
          @click.stop="moveUp()"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-orange-500 p-0 text-white shadow hover:bg-orange-600 active:shadow-lg"
          title="Move to parent"
        >
          <ArrowUpIcon :size="20" />
          <KbdHint contents="U" :show="showHint && isSelected" />
        </button>
        <button
          @click.stop="emit('moveCancelled')"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-red-500 p-0 text-white shadow hover:bg-red-600 active:shadow-lg"
          title="Cancel"
        >
          <CloseIcon :size="20" />
          <KbdHint contents="Esc" :show="showHint && isSelected" />
        </button>
      </div>
    </div>
    <!-- Match badges -->
    <div class="absolute top-2 right-2 flex items-center gap-1">
      <div
        v-if="recordsStore.apiSearchScores[appRecord.ID]?.text != null"
        class="cursor-default rounded bg-gray-100 px-1 text-xs text-gray-400 dark:bg-gray-700"
        :title="`Text search: ${(recordsStore.apiSearchScores[appRecord.ID]!.text! * 100).toFixed(1)}%`"
      >
        {{
          Math.round(recordsStore.apiSearchScores[appRecord.ID]!.text! * 100)
        }}%T
      </div>
      <div
        v-if="
          recordsStore.apiSearchScores[appRecord.ID]?.image != null &&
          recordsStore.apiSearchScores[appRecord.ID]!.image! > 0
        "
        class="cursor-default rounded bg-gray-100 px-1 text-xs text-gray-400 dark:bg-gray-700"
        :title="`Image search: ${(recordsStore.apiSearchScores[appRecord.ID]!.image! * 100).toFixed(1)}%`"
      >
        {{
          Math.round(recordsStore.apiSearchScores[appRecord.ID]!.image! * 100)
        }}%I
      </div>
    </div>

    <!-- Content -->
    <div class="flex flex-auto flex-col p-4">
      <!-- Title -->
      <div v-if="!editMode">
        <div
          class="list-reset mb-2 flex cursor-pointer items-baseline space-x-3"
          @click.stop="recordsStore.setCurrentRecord(appRecord.ID)"
        >
          <div class="text-xl font-bold" :title="`ID: ${appRecord.ID}`">
            <template v-if="recordsStore.searchtext.trim()">
              <template
                v-for="(seg, i) in formatOptionSegments(appRecord.ID)"
                :key="i"
                ><span v-if="i > 0">/</span
                ><span
                  :class="
                    seg.isRef
                      ? 'font-mono text-blue-600 dark:text-blue-400'
                      : ''
                  "
                  >{{ seg.text }}</span
                ></template
              >
            </template>
            <template v-else>
              <span
                v-if="
                  appRecord.Title &&
                  appRecord.Title !== appRecord.ReferenceNumber
                "
                class="inline-flex items-baseline gap-1"
              >
                <span>{{
                  appRecord.Quantity
                    ? `${appRecord.Title} (x${appRecord.Quantity})`
                    : appRecord.Title
                }}</span>
              </span>
              <span
                v-else-if="!appRecord.ReferenceNumber"
                class="font-normal text-gray-400 dark:text-gray-500"
                >({{ appRecord.ID }})</span
              >
            </template>
          </div>
          <div
            v-if="!recordsStore.searchtext.trim() && appRecord.ReferenceNumber"
            class="font-mono text-xl text-blue-600 dark:text-blue-400"
          >
            #{{ appRecord.ReferenceNumber }}
          </div>
          <span
            v-if="
              /^\d+$/.test(appRecord.Title as string) &&
              ((appRecord.ReferenceNumber &&
                appRecord.Title !== appRecord.ReferenceNumber) ||
                (!appRecord.ReferenceNumber &&
                  parseInt(appRecord.Title as string, 10) !== appRecord.ID))
            "
            class="relative flex"
          >
            <AlertIcon
              class="self-center text-yellow-500"
              :size="18"
              title="Name mismatch"
            />
            <KbdHint
              :contents="
                appRecord.ReferenceNumber &&
                appRecord.Title !== appRecord.ReferenceNumber
                  ? 'Name/Ref'
                  : appRecord.Title &&
                      parseInt(appRecord.Title, 10) !== appRecord.ID
                    ? 'Name/ID'
                    : ''
              "
              :show="showHint"
              :inline="true"
            />
          </span>
        </div>
      </div>

      <!-- Edit mode title -->
      <div v-else>
        <div class="list-reset mb-2 flex flex-auto items-baseline space-x-2">
          <input
            ref="nameInputEl"
            type="text"
            v-model="localRecord.Title"
            class="rounded-sm bg-white ring-1 dark:bg-gray-900"
            placeholder="Name"
          />
          <AlertIcon
            v-if="!localRecord.ReferenceNumber && nameIsWrongNumber"
            class="shrink-0 self-center text-yellow-500"
            :size="20"
            :title="
              nameRefMismatch
                ? 'Name and reference number don\'t match'
                : 'Name is a number that doesn\'t match this record\'s ID'
            "
          />
          <input
            type="text"
            v-model="localRecord.ReferenceNumber"
            class="w-16 rounded-sm bg-white font-mono ring-1 dark:bg-gray-900"
            :placeholder="nextRefPlaceholder ?? 'Ref#'"
          />
          <span v-if="refTaken || nameRefMismatch" class="relative flex">
            <AlertIcon
              class="shrink-0 self-center text-yellow-500"
              :size="20"
              :title="
                nameRefMismatch
                  ? 'Name and reference number don\'t match'
                  : 'Reference number already in use'
              "
            />
            <KbdHint contents="Name/Ref" :show="showHint" :inline="true" />
          </span>
          <input
            type="number"
            min="0"
            v-model.number="localRecord.Quantity"
            class="w-10 rounded-sm bg-white ring-1 dark:bg-gray-900"
            placeholder="Qty"
          />
        </div>
      </div>

      <!-- Description -->
      <div v-if="!editMode && appRecord.description">
        <p class="text-gray-600 dark:text-gray-400">
          {{ appRecord.description }}
        </p>
      </div>
      <div v-else-if="editMode">
        <textarea
          v-model="localRecord.description"
          class="w-full rounded-sm bg-white ring-1 dark:bg-gray-900"
          rows="3"
          placeholder="Description"
        ></textarea>
      </div>

      <!-- Children -->
      <div v-if="!editMode && recordsStore.hasChildren(appRecord.ID)">
        <p class="mb-2 font-semibold">Contains:</p>
        <div
          class="flex max-h-32 flex-wrap gap-2 overflow-hidden rounded-md p-2 shadow-md ring-1 ring-gray-500/10 hover:overflow-y-auto hover:shadow-lg hover:ring-gray-500/25"
          style="scrollbar-gutter: stable"
          @dragenter="isDragOverChildren = true"
          @dragleave="
            (e) => {
              if (
                !(e.currentTarget as HTMLElement).contains(
                  e.relatedTarget as Node,
                )
              )
                isDragOverChildren = false;
            }
          "
        >
          <div
            v-for="childId in recordsStore.listChildLocations(appRecord.ID)"
            :key="childId"
            draggable="true"
            :class="[
              'cursor-pointer rounded bg-gray-50 p-1 ring-1 transition-colors active:shadow-md dark:bg-gray-800',
              childDragReadyId === childId
                ? 'bg-green-50 shadow ring-2 shadow-green-200 ring-green-500 dark:bg-green-900/20 dark:shadow-green-900'
                : 'ring-gray-200 hover:bg-gray-100 hover:shadow-sm hover:ring-blue-500/75 dark:ring-slate-500 dark:hover:bg-gray-700',
              draggingChildId === childId ? 'opacity-40' : '',
            ]"
            @click.stop="recordsStore.setCurrentRecord(childId)"
            @dragstart="handleChildDragStart($event, childId)"
            @dragend="handleChildDragEnd"
            @dragover="handleChildDragOver($event, childId)"
            @dragleave="handleChildDragLeave"
            @drop="handleChildDrop($event, childId)"
          >
            <template v-if="recordsStore.recordMap[childId]?.ReferenceNumber">
              <span class="font-mono text-blue-600 dark:text-blue-400"
                >#{{ recordsStore.recordMap[childId]!.ReferenceNumber }}</span
              >
            </template>
            <template v-else-if="recordsStore.recordMap[childId]?.Title">{{
              recordsStore.recordMap[childId]!.Title
            }}</template>
            <span v-else class="font-normal text-gray-400 dark:text-gray-500"
              >({{ childId }})</span
            >
          </div>
        </div>
      </div>
    </div>

    <!-- Action buttons -->
    <div class="flex flex-wrap gap-2 p-4">
      <template v-if="!editMode">
        <button
          @click.stop="emit('requestDelete')"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-red-500 p-0 text-white shadow hover:bg-red-600 active:shadow-lg"
          title="Delete record"
        >
          <TrashCanIcon :size="20" />
          <KbdHint contents="Del" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="emit('requestMove', appRecord.ID)"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Move record"
        >
          <FolderMoveIcon :size="20" />
          <KbdHint contents="M" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="handleEditToggle"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Edit record"
        >
          <PencilIcon :size="20" />
          <KbdHint contents="Enter" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="handleQuickCapture"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Quick capture (add photo to this record)"
        >
          <CameraIcon :size="20" />
          <KbdHint contents="P" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="handleQuickCaptureNewChild"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Quick capture new child record"
        >
          <CameraPlusIcon :size="20" />
          <kbdHint contents="⇧C" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="handleSearchByImage"
          v-if="appRecord.Artifacts && appRecord.Artifacts.length > 0"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-purple-500 p-0 text-white shadow hover:bg-purple-600 active:shadow-lg"
          title="Search for similar records"
        >
          <ImageSearchIcon :size="20" />
          <kbdHint contents="⇧S" :show="showHint && isSelected" />
        </button>

        <button
          @click.stop="emit('createChild', appRecord.ID)"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="New record as child"
        >
          <PlusIcon :size="20" />
          <kbdHint contents="⇧N" :show="showHint && isSelected" />
        </button>
      </template>

      <template v-else>
        <button
          @click.stop="handleSave"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Save"
        >
          <CheckIcon :size="20" />
          <KbdHint contents="Enter" :show="showHint" />
        </button>

        <button
          @click.stop="handleCancel"
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-red-500 p-0 text-white shadow hover:bg-red-600 active:shadow-lg"
          title="Cancel"
        >
          <CloseIcon :size="20" />
          <KbdHint contents="Esc" :show="showHint" />
        </button>

        <button
          @click.stop="
            cameraStore.open((files: File[]) =>
              files.forEach((f) => handleEditArtifact(f)),
            )
          "
          class="relative m-0 flex h-10 w-10 items-center justify-center rounded-full bg-blue-500 p-0 text-white shadow hover:bg-blue-600 active:shadow-lg"
          title="Capture artifact"
        >
          <CameraIcon :size="20" />
          <KbdHint contents="P" :show="showHint" />
        </button>
      </template>
    </div>

    <!-- Images -->
    <div class="flex w-full flex-row justify-center">
      <template v-if="!editMode && images.length > 0">
        <template v-for="n in images" :key="n">
          <ArtifactImage
            class="h-56 w-full flex-1 rounded-xl object-cover"
            :artifact-id="n"
            :alt="`Artifact ${n}`"
          />
        </template>
      </template>

      <template v-else-if="editMode && images.length > 0">
        <template v-for="n in images" :key="n">
          <div class="relative flex-1">
            <ArtifactImage
              class="h-56 w-full rounded-xl object-cover transition-opacity"
              :class="pendingDeletions.has(n) ? 'opacity-30' : 'opacity-100'"
              :artifact-id="n"
              :alt="`Artifact ${n}`"
            />
            <button
              type="button"
              @click.stop="toggleArtifactDeletion(n)"
              class="absolute top-1 right-1 flex h-6 w-6 items-center justify-center rounded-full text-sm leading-none text-white transition-colors"
              :class="
                pendingDeletions.has(n)
                  ? 'bg-gray-400 hover:bg-gray-500'
                  : 'bg-red-500 hover:bg-red-600'
              "
              :title="
                pendingDeletions.has(n) ? 'Undo removal' : 'Remove artifact'
              "
            >
              <CloseIcon :size="16" />
            </button>
          </div>
        </template>
      </template>
    </div>
  </figure>
</template>
