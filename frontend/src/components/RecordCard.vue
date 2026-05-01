<script setup lang="ts" name="RecordCard">
import { ref, computed, watch, nextTick, onUnmounted } from "vue";
import { useRecordsStore } from "@/stores/records";
import { useCameraStore } from "@/stores/camera";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";
import type { AppRecord } from "@/api/types";
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
    appRecord: AppRecord;
    isSelected?: boolean;
    showHint?: boolean;
    startEdit?: boolean;
    confirmDelete?: boolean;
    confirmMove?: boolean;
}>();

const emit = defineEmits<{
    recordUpdated: [appRecord: AppRecord];
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
const localRecord = ref<AppRecord>({
    ...props.appRecord,
    metadata: { ...props.appRecord.metadata },
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
    if ((e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) return;
    if (childDragTimer !== null) { clearTimeout(childDragTimer); childDragTimer = null; }
    childDragReadyId.value = null;
};

const handleChildDrop = async (e: DragEvent, childId: number): Promise<void> => {
    e.stopPropagation();
    if (childDragTimer !== null) { clearTimeout(childDragTimer); childDragTimer = null; }
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
    () => !pointerOnEditable.value && !editMode.value && !props.confirmDelete && !props.confirmMove,
);

const handlePointerDown = (e: PointerEvent): void => {
    pointerOnEditable.value = !!(e.target as HTMLElement).closest("input, textarea, [contenteditable]");
};

const handlePointerUp = (): void => {
    pointerOnEditable.value = false;
};

const handleDragStart = (e: DragEvent): void => {
    const el = cardEl.value;
    if (!el) return;
    e.dataTransfer?.setData("recordId", props.appRecord.id.toString());
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
    if (isNaN(draggedId) || draggedId === props.appRecord.id) return;
    if (isDescendantOf(props.appRecord.id, draggedId)) return;
    // Child dragged back over its own children box — leave it where it is
    if (draggingChildId.value !== null && isDragOverChildren.value) return;
    // Child being dragged out → move to this card's parent level, not into the card
    const targetId = draggingChildId.value !== null ? props.appRecord.location : props.appRecord.id;
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
    const n = localRecord.value.name;
    return !!n && /^\d+$/.test(n) && parseInt(n, 10) !== props.appRecord.id;
});

const nameRefMismatch = computed(() => {
    const n = localRecord.value.name;
    const r = localRecord.value.metadata.referenceNumber;
    return !!n && !!r && /^\d+$/.test(n) && /^\d+$/.test(r) && n !== r;
});

const refTaken = computed(() => {
    const v = localRecord.value.metadata.referenceNumber?.trim();
    if (!v) return false;
    return Object.values(recordsStore.recordMap).some(
        (e) => e.id !== props.appRecord.id && e.metadata.referenceNumber === v,
    );
});

watch(editMode, async (on) => {
    if (on) {
        nextRefPlaceholder.value = String(await api.nextReferenceNumber([localRecord.value.id]));
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
        current = parent.location;
    }
    return false;
};

const moveUp = (): void => {
    if (recordsStore.currentRecord !== 0) {
        const currentRec =
            recordsStore.recordMap[recordsStore.currentRecord];
        if (currentRec?.location !== undefined) {
            emit("moveConfirmed", currentRec.location);
        }
    }
};

const filteredMoveRecords = computed(() => {
    const term = recordsStore.moveSearchtext.toLowerCase().trim();
    const world: AppRecord = {
        id: 0,
        name: "World",
        description: "",
        artifacts: [],
        location: 0,
        metadata: {
            quantity: null,
            owner: null,
            lastModified: null,
            referenceNumber: null,
        },
    };
    const candidates: AppRecord[] = [
        ...Object.values(recordsStore.recordMap).filter(
            (e) =>
                e.id !== props.appRecord.id &&
                !isDescendantOf(e.id, props.appRecord.id),
        ),
        world,
    ];
    if (!term) return candidates;
    return candidates.filter(
        (e) =>
            e.name?.toLowerCase().includes(term) ||
            e.description?.toLowerCase().includes(term) ||
            e.id.toString().includes(term) ||
            e.metadata.referenceNumber?.toString().includes(term),
    );
});

const currentLocationName = computed(() => {
    if (recordsStore.currentRecord === 0) return "World";
    return recordsStore.readname(recordsStore.currentRecord);
});

const isAtCurrentLocation = computed((): boolean => {
    if (props.appRecord.location === undefined) return false;
    return props.appRecord.location === recordsStore.currentRecord;
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
    if ((e.key === "h" || e.key === "H")) {
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
                    ? (results[0]?.id ?? 0)
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
    if (results.some((r) => r.id === moveTargetLocation.value)) return;
    const term = recordsStore.moveSearchtext.toLowerCase().trim();
    const byRef = term && results.find((r) => r.metadata.referenceNumber?.toLowerCase() === term);
    const byId = term && results.find((r) => r.id.toString() === term);
    moveTargetLocation.value = (byRef || byId || results[0])?.id ?? 0;
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
        if (elem.name) {
            tree.push({ text: elem.name, isRef: false });
        } else if (elem.metadata.referenceNumber) {
            tree.push({
                text: `#${elem.metadata.referenceNumber}`,
                isRef: true,
            });
        } else {
            tree.push({ text: target.toString(), isRef: false });
        }
        target = elem.location;
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
        const artifacts = (localRecord.value.artifacts ?? []).filter(
            (id) => !pendingDeletions.value.has(id),
        );
        await api.updateRecord(props.appRecord.id, {
            Title: e.name || null,
            ReferenceNumber: e.metadata.referenceNumber || null,
            Description: e.description,
            Quantity: typeof e.metadata.quantity === "number" ? e.metadata.quantity : null,
            ParentID: e.location || undefined,
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
        await api.deleteRecord(props.appRecord.id);
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
        const artifacts = [...(props.appRecord.artifacts ?? []), artifactId];
        await api.patchRecord(props.appRecord.id, { Artifacts: artifacts });
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
    if (!props.appRecord.artifacts || props.appRecord.artifacts.length === 0) return;

    try {
        const artifactId = props.appRecord.artifacts[0];
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
                await api.patchRecord(props.appRecord.id, { Artifacts: [artifactId] });
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
            metadata: { ...props.appRecord.metadata },
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
        metadata: { ...props.appRecord.metadata },
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
        ? localRecord.value.artifacts
        : props.appRecord.artifacts;
    return artifacts ?? [];
});

const handleEditArtifact = async (file: File): Promise<void> => {
    try {
        const artifactId = await api.uploadArtifact(file);
        const artifacts = [...(localRecord.value.artifacts ?? []), artifactId];
        localRecord.value = { ...localRecord.value, artifacts };
        await api.updateRecord(props.appRecord.id, { Artifacts: artifacts });
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
    <figure ref="cardEl" :draggable="isDraggable"
        class="relative h-full min-h-48 w-sm xs:w-full bg-white shadow-md dark:bg-gray-800 rounded-xl flex flex-col cursor-default transition-opacity"
        :class="[
            isSelected
                ? 'ring-2 ring-blue-500 shadow-blue-200 dark:shadow-blue-900'
                : isDragOver
                    ? draggingChildId !== null && !isDragOverChildren
                        ? 'ring-2 ring-blue-400 shadow-blue-100 dark:shadow-blue-900/30 bg-blue-50/50 dark:bg-blue-900/10'
                        : childDragReadyId !== null
                            ? 'ring-2 ring-green-300 dark:ring-green-800 bg-green-50/50 dark:bg-green-900/10'
                            : 'ring-2 ring-green-500 shadow-green-200 dark:shadow-green-900 bg-green-50 dark:bg-green-900/20'
                    : 'ring-1 ring-gray-500/25 hover:ring-gray-500/50 hover:shadow-lg',
            isDragging ? 'opacity-40' : '',
        ]" @click="emit('select')" @pointerdown="handlePointerDown" @pointerup="handlePointerUp"
        @dragstart="handleDragStart" @dragend="handleDragEnd" @dragover="handleDragOver" @dragleave="handleDragLeave"
        @drop="handleDrop">
        <!-- Delete confirmation overlay -->
        <div v-if="confirmDelete"
            class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 rounded-xl bg-white/90 dark:bg-gray-800/90 backdrop-blur-sm"
            @click.stop>
            <p class="text-lg font-semibold text-red-600 dark:text-red-400">
                {{ appRecord.name || appRecord.metadata.referenceNumber ? `Delete "${appRecord.name ||
                    `#${appRecord.metadata.referenceNumber}`}"?` : "Delete?" }}
            </p>
            <div class="flex gap-3">
                <button @click.stop="emit('deleteConfirmed')"
                    class="h-9 px-4 rounded-full bg-red-500 hover:bg-red-600 text-white text-sm shadow relative">
                    Delete
                    <KbdHint contents="Enter" :show="showHint && isSelected" />
                </button>
                <button @click.stop="emit('deleteCancelled')"
                    class="h-9 px-4 rounded-full bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-sm shadow relative">
                    Cancel
                    <KbdHint contents="Esc" :show="showHint && isSelected" />
                </button>
            </div>
        </div>
        <!-- Move confirmation overlay -->
        <div v-if="confirmMove"
            class="absolute inset-0 z-10 flex flex-col gap-2 rounded-xl bg-white/95 dark:bg-gray-800/95 backdrop-blur-sm p-4"
            @click.stop>
            <p class="text-sm font-semibold text-gray-700 dark:text-gray-300">
                {{ appRecord.name || appRecord.metadata.referenceNumber ? `Move "${appRecord.name ||
                    `#${appRecord.metadata.referenceNumber}`}" to:` : "Move to:" }}
            </p>
            <input ref="moveSearchInputRef" v-model="recordsStore.moveSearchtext" type="search"
                placeholder="Search locations..."
                class="w-full px-3 py-1.5 rounded-full bg-white ring-1 dark:bg-gray-900 text-sm" @click.stop />
            <select v-model="moveTargetLocation"
                class="w-full px-2 py-1 rounded-lg bg-white ring-1 dark:bg-gray-900 text-sm flex-1 min-h-0" size="4"
                @click.stop>
                <option v-for="loc in filteredMoveRecords" :key="loc.id" :value="loc.id">
                    {{
                        formatOptionSegments(loc.id)
                            .map((s) => s.text)
                            .join("/")
                    }}
                </option>
            </select>
            <div class="flex gap-2 flex-wrap items-center">
                <button @click.stop="emit('moveConfirmed', moveTargetLocation)"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Move">
                    <CheckIcon :size="20" />
                    <KbdHint contents="Enter" :show="showHint && isSelected" />
                </button>
                <button v-if="!isAtCurrentLocation" @click.stop="
                    emit('moveConfirmed', recordsStore.currentRecord)
                    "
                    class="h-10 px-3 rounded-full bg-purple-500 hover:bg-purple-600 text-white text-sm shadow relative">
                    To {{ currentLocationName }}
                    <KbdHint contents="H" :show="showHint && isSelected" />
                </button>
                <button v-if="appRecord.id !== 0 && recordsStore.currentRecord !== 0" @click.stop="moveUp()"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-orange-500 rounded-full shadow hover:bg-orange-600 active:shadow-lg text-white"
                    title="Move to parent">
                    <ArrowUpIcon :size="20" />
                    <KbdHint contents="U" :show="showHint && isSelected" />
                </button>
                <button @click.stop="emit('moveCancelled')"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-red-500 rounded-full shadow hover:bg-red-600 active:shadow-lg text-white"
                    title="Cancel">
                    <CloseIcon :size="20" />
                    <KbdHint contents="Esc" :show="showHint && isSelected" />
                </button>
            </div>
        </div>
        <!-- Match badges -->
        <div class="absolute top-2 right-2 flex gap-1 items-center">
            <div v-if="recordsStore.apiSearchScores[appRecord.id]?.text != null"
                class="text-xs text-gray-400 px-1 rounded bg-gray-100 dark:bg-gray-700 cursor-default"
                :title="`Text search: ${(recordsStore.apiSearchScores[appRecord.id]!.text! * 100).toFixed(1)}%`">
                {{
                    Math.round(
                        recordsStore.apiSearchScores[appRecord.id]!.text! * 100,
                    )
                }}%T
            </div>
            <div v-if="
                recordsStore.apiSearchScores[appRecord.id]?.image != null &&
                recordsStore.apiSearchScores[appRecord.id]!.image! > 0
            " class="text-xs text-gray-400 px-1 rounded bg-gray-100 dark:bg-gray-700 cursor-default"
                :title="`Image search: ${(recordsStore.apiSearchScores[appRecord.id]!.image! * 100).toFixed(1)}%`">
                {{
                    Math.round(
                        recordsStore.apiSearchScores[appRecord.id]!.image! * 100,
                    )
                }}%I
            </div>
        </div>

        <!-- Content -->
        <div class="p-4 flex-auto flex flex-col">
            <!-- Title -->
            <div v-if="!editMode">
                <div class="flex list-reset space-x-3 items-baseline mb-2 cursor-pointer"
                    @click.stop="recordsStore.setCurrentRecord(appRecord.id)">
                    <div class="text-xl font-bold" :title="`ID: ${appRecord.id}`">
                        <template v-if="recordsStore.searchtext.trim()">
                            <template v-for="(seg, i) in formatOptionSegments(
                                appRecord.id,
                            )" :key="i"><span v-if="i > 0">/</span><span :class="seg.isRef
                                ? 'font-mono text-blue-600 dark:text-blue-400'
                                : ''
                                ">{{ seg.text }}</span></template>
                        </template>
                        <template v-else>
                            <span v-if="appRecord.name && appRecord.name !== appRecord.metadata.referenceNumber"
                                class="inline-flex items-baseline gap-1">
                                <span>{{
                                    appRecord.metadata.quantity
                                        ? `${appRecord.name} (x${appRecord.metadata.quantity})`
                                        : appRecord.name
                                }}</span>
                            </span>
                            <span v-else-if="!appRecord.metadata.referenceNumber"
                                class="font-normal text-gray-400 dark:text-gray-500">({{ appRecord.id
                                }})</span>
                        </template>
                    </div>
                    <div v-if="
                        !recordsStore.searchtext.trim() &&
                        appRecord.metadata.referenceNumber
                    " class="text-xl font-mono text-blue-600 dark:text-blue-400">
                        #{{ appRecord.metadata.referenceNumber }}
                    </div>
                    <span v-if="
                        /^\d+$/.test(appRecord.name as string) &&
                        (
                            (
                                appRecord.metadata.referenceNumber &&
                                appRecord.name !== appRecord.metadata.referenceNumber
                            ) ||
                            (
                                !appRecord.metadata.referenceNumber &&
                                parseInt(appRecord.name as string, 10) !== appRecord.id
                            )
                        )" class="relative flex">
                        <AlertIcon class="text-yellow-500 self-center" :size="18" title="Name mismatch" />
                        <KbdHint
                            :contents="appRecord.metadata.referenceNumber && appRecord.name !== appRecord.metadata.referenceNumber ? 'Name/Ref' : appRecord.name && parseInt(appRecord.name, 10) !== appRecord.id ? 'Name/ID' : ''"
                            :show="showHint" :inline="true" />
                    </span>
                </div>
            </div>

            <!-- Edit mode title -->
            <div v-else>
                <div class="flex-auto flex list-reset space-x-2 items-baseline mb-2">
                    <input ref="nameInputEl" type="text" v-model="localRecord.name"
                        class="bg-white rounded-sm dark:bg-gray-900 ring-1" placeholder="Name" />
                    <AlertIcon v-if="!localRecord.metadata.referenceNumber && nameIsWrongNumber"
                        class="text-yellow-500 self-center shrink-0" :size="20"
                        :title="nameRefMismatch ? 'Name and reference number don\'t match' : 'Name is a number that doesn\'t match this record\'s ID'" />
                    <input type="text" v-model="localRecord.metadata.referenceNumber"
                        class="bg-white rounded-sm dark:bg-gray-900 ring-1 w-16 font-mono"
                        :placeholder="nextRefPlaceholder ?? 'Ref#'" />
                    <span v-if="refTaken || nameRefMismatch" class="relative flex">
                        <AlertIcon class="text-yellow-500 self-center shrink-0" :size="20"
                            :title="nameRefMismatch ? 'Name and reference number don\'t match' : 'Reference number already in use'" />
                        <KbdHint contents="Name/Ref" :show="showHint" :inline="true" />
                    </span>
                    <input type="number" min="0" v-model.number="localRecord.metadata.quantity"
                        class="bg-white rounded-sm dark:bg-gray-900 ring-1 w-10" placeholder="Qty" />
                </div>
            </div>

            <!-- Description -->
            <div v-if="!editMode && appRecord.description">
                <p class="text-gray-600 dark:text-gray-400">
                    {{ appRecord.description }}
                </p>
            </div>
            <div v-else-if="editMode">
                <textarea v-model="localRecord.description" class="bg-white rounded-sm dark:bg-gray-900 ring-1 w-full"
                    rows="3" placeholder="Description"></textarea>
            </div>

            <!-- Children -->
            <div v-if="!editMode && recordsStore.hasChildren(appRecord.id)">
                <p class="mb-2 font-semibold">Contains:</p>
                <div class="flex flex-wrap gap-2 overflow-hidden hover:overflow-y-auto max-h-32 shadow-md p-2 ring-1 ring-gray-500/10 hover:ring-gray-500/25 hover:shadow-lg rounded-md"
                    style="scrollbar-gutter: stable" @dragenter="isDragOverChildren = true"
                    @dragleave="(e) => { if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) isDragOverChildren = false; }">
                    <div v-for="childId in recordsStore.listChildLocations(
                        appRecord.id,
                    )" :key="childId" draggable="true" :class="[
                        'p-1 rounded cursor-pointer bg-gray-50 dark:bg-gray-800 ring-1 active:shadow-md transition-colors',
                        childDragReadyId === childId
                            ? 'ring-2 ring-green-500 shadow shadow-green-200 dark:shadow-green-900 bg-green-50 dark:bg-green-900/20'
                            : 'dark:hover:bg-gray-700 hover:bg-gray-100 hover:shadow-sm ring-gray-200 dark:ring-slate-500 hover:ring-blue-500/75',
                        draggingChildId === childId ? 'opacity-40' : '',
                    ]" @click.stop="recordsStore.setCurrentRecord(childId)"
                        @dragstart="handleChildDragStart($event, childId)" @dragend="handleChildDragEnd"
                        @dragover="handleChildDragOver($event, childId)" @dragleave="handleChildDragLeave"
                        @drop="handleChildDrop($event, childId)">
                        <template v-if="recordsStore.recordMap[childId]?.metadata.referenceNumber">
                            <span class="font-mono text-blue-600 dark:text-blue-400">#{{
                                recordsStore.recordMap[childId]!.metadata.referenceNumber }}</span>
                        </template>
                        <template v-else-if="recordsStore.recordMap[childId]?.name">{{
                            recordsStore.recordMap[childId]!.name }}</template>
                        <span v-else class="font-normal text-gray-400 dark:text-gray-500">({{ childId }})</span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Action buttons -->
        <div class="p-4 flex flex-wrap gap-2">
            <template v-if="!editMode">
                <button @click.stop="emit('requestDelete')"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-red-500 rounded-full shadow hover:bg-red-600 active:shadow-lg text-white"
                    title="Delete record">
                    <TrashCanIcon :size="20" />
                    <KbdHint contents="Del" :show="showHint && isSelected" />
                </button>

                <button @click.stop="emit('requestMove', appRecord.id)"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Move record">
                    <FolderMoveIcon :size="20" />
                    <KbdHint contents="M" :show="showHint && isSelected" />
                </button>

                <button @click.stop="handleEditToggle"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Edit record">
                    <PencilIcon :size="20" />
                    <KbdHint contents="Enter" :show="showHint && isSelected" />
                </button>

                <button @click.stop="handleQuickCapture"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Quick capture (add photo to this record)">
                    <CameraIcon :size="20" />
                    <KbdHint contents="P" :show="showHint && isSelected" />
                </button>

                <button @click.stop="handleQuickCaptureNewChild"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Quick capture new child record">
                    <CameraPlusIcon :size="20" />
                    <kbdHint contents="⇧C" :show="showHint && isSelected" />
                </button>

                <button @click.stop="handleSearchByImage" v-if="appRecord.artifacts && appRecord.artifacts.length > 0"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-purple-500 rounded-full shadow hover:bg-purple-600 active:shadow-lg text-white"
                    title="Search for similar records">
                    <ImageSearchIcon :size="20" />
                    <kbdHint contents="⇧S" :show="showHint && isSelected" />
                </button>

                <button @click.stop="emit('createChild', appRecord.id)"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="New record as child">
                    <PlusIcon :size="20" />
                    <kbdHint contents="⇧N" :show="showHint && isSelected" />
                </button>
            </template>

            <template v-else>
                <button @click.stop="handleSave"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Save">
                    <CheckIcon :size="20" />
                    <KbdHint contents="Enter" :show="showHint" />
                </button>

                <button @click.stop="handleCancel"
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-red-500 rounded-full shadow hover:bg-red-600 active:shadow-lg text-white"
                    title="Cancel">
                    <CloseIcon :size="20" />
                    <KbdHint contents="Esc" :show="showHint" />
                </button>

                <button @click.stop="
                    cameraStore.open((files: File[]) =>
                        files.forEach((f) => handleEditArtifact(f)),
                    )
                    "
                    class="relative h-10 w-10 p-0 m-0 flex items-center justify-center bg-blue-500 rounded-full shadow hover:bg-blue-600 active:shadow-lg text-white"
                    title="Capture artifact">
                    <CameraIcon :size="20" />
                    <KbdHint contents="P" :show="showHint" />
                </button>
            </template>
        </div>

        <!-- Images -->
        <div class="flex flex-row justify-center w-full">
            <template v-if="!editMode && images.length > 0">
                <template v-for="n in images" :key="n">
                    <ArtifactImage class="flex-1 object-cover w-full h-56 rounded-xl" :artifact-id="n"
                        :alt="`Artifact ${n}`" />
                </template>
            </template>

            <template v-else-if="editMode && images.length > 0">
                <template v-for="n in images" :key="n">
                    <div class="relative flex-1">
                        <ArtifactImage class="object-cover w-full h-56 rounded-xl transition-opacity" :class="pendingDeletions.has(n)
                            ? 'opacity-30'
                            : 'opacity-100'
                            " :artifact-id="n" :alt="`Artifact ${n}`" />
                        <button type="button" @click.stop="toggleArtifactDeletion(n)"
                            class="absolute top-1 right-1 w-6 h-6 flex items-center justify-center rounded-full text-white text-sm leading-none transition-colors"
                            :class="pendingDeletions.has(n)
                                ? 'bg-gray-400 hover:bg-gray-500'
                                : 'bg-red-500 hover:bg-red-600'
                                " :title="pendingDeletions.has(n)
                                    ? 'Undo removal'
                                    : 'Remove artifact'
                                    ">
                            <CloseIcon :size="16" />
                        </button>
                    </div>
                </template>
            </template>
        </div>
    </figure>
</template>
