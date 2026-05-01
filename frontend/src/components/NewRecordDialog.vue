<script setup lang="ts" name="NewRecordDialog">
import { ref, computed, watch, nextTick } from "vue";
import AlertIcon from "vue-material-design-icons/Alert.vue";
import KbdHint from "@/components/KbdHint.vue";
import { useRecordsStore } from "@/stores/records";
import { useCameraStore } from "@/stores/camera";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";

const recordsStore = useRecordsStore();
const cameraStore = useCameraStore();
const toastsStore = useToastsStore();

const props = withDefaults(
  defineProps<{
    visible?: boolean;
    location?: number;
    showHint?: boolean;
  }>(),
  {
    visible: false,
    location: 0,
    showHint: false,
  },
);

const emit = defineEmits<{
  created: [recordId: number];
  "update:visible": [value: boolean];
}>();

const dialogVisible = ref(false);
const nameInput = ref<HTMLInputElement | null>(null);
const title = ref<string>("");
const description = ref<string>("");
const quantity = ref<number | null>(null);
const referenceNumber = ref<string>("");
const files = ref<File[]>([]);
const nextRefNumber = ref<number>(0);

const refTaken = computed(() => {
  const v = referenceNumber.value.trim();
  if (!v) return false;
  return Object.values(recordsStore.recordMap).some(
    (e) => e.metadata.referenceNumber === v,
  );
});

watch(
  () => props.visible,
  async (visible) => {
    dialogVisible.value = visible;
    if (visible) {
      resetDialog();
      nextRefNumber.value = await api.nextReferenceNumber();
      nextTick(() => nameInput.value?.focus());
    }
  },
  { immediate: true },
);

const resetDialog = (): void => {
  title.value = "";
  description.value = "";
  quantity.value = null;
  referenceNumber.value = "";
  files.value = [];
  nextRefNumber.value = 0;
};

const handleSubmit = async (): Promise<void> => {
  if (refTaken.value) {
    toastsStore.add("Conflicting Reference #", "warn");
    return;
  }
  try {
    let artifactIds: number[] = [];
    for (const file of files.value) {
      const id = await api.uploadArtifact(file);
      artifactIds.push(id);
    }

    const record = await api.createRecord({
      Title: title.value || null,
      ReferenceNumber: referenceNumber.value || null,
      Description: description.value || null,
      Quantity: typeof quantity.value === "number" ? quantity.value : null,
      ParentID: props.location || undefined,
      Artifacts: artifactIds.length ? artifactIds : undefined,
    });
    await recordsStore.reload();
    emit("created", record.ID);
    emit("update:visible", false);
    dialogVisible.value = false;
    toastsStore.add("Record created", "success");
  } catch (error) {
    console.error("Failed to create record:", error);
    toastsStore.add("Failed to create record");
  }
};

const handleDialogClose = (): void => {
  dialogVisible.value = false;
  emit("update:visible", false);
};

const handleCameraOpen = async (): Promise<void> => {
  await new Promise<void>((resolve) => {
    cameraStore.open((fileList: File[]) => {
      files.value = fileList;
      resolve();
    });
  });
};
</script>

<template>
  <Teleport to="body">
    <div
      v-if="dialogVisible"
      class="fixed inset-0 z-50 overflow-y-auto"
      role="dialog"
      aria-modal="true"
    >
      <!-- Overlay -->
      <div class="fixed inset-0 bg-black/40" @click="handleDialogClose"></div>

      <!-- Panel -->
      <div
        class="relative flex min-h-screen items-center justify-center p-4"
        @click.stop
        @keydown.esc.stop="handleDialogClose"
      >
        <div
          class="relative w-full overflow-y-auto rounded-lg border border-gray-300 bg-white p-8 dark:bg-gray-800"
        >
          <!-- Title -->
          <h1 class="pb-4 text-3xl font-medium">Create New Record</h1>

          <!-- Form -->
          <div
            class="xs:grid-cols-1 grid items-center gap-x-4 gap-y-3 sm:grid-cols-[6rem_1fr]"
          >
            <label for="name">Name</label>
            <input
              id="name"
              ref="nameInput"
              type="text"
              v-model="title"
              class="rounded-sm bg-white px-2 py-1 ring-1 dark:bg-gray-900"
              @keydown.enter.prevent="handleSubmit"
            />

            <label for="refnum">Reference #</label>
            <div class="flex items-center gap-2">
              <input
                id="refnum"
                type="text"
                v-model="referenceNumber"
                class="rounded-sm bg-white px-2 py-1 ring-1 dark:bg-gray-900"
                :placeholder="String(nextRefNumber)"
                @keydown.enter.prevent="handleSubmit"
              />
              <AlertIcon
                v-if="refTaken"
                class="shrink-0 text-yellow-500"
                :size="20"
                title="Reference number already in use"
              />
            </div>

            <label for="description">Description</label>
            <textarea
              id="description"
              v-model="description"
              class="rounded-sm bg-white px-2 py-1 ring-1 dark:bg-gray-900"
              rows="3"
            ></textarea>

            <label for="quantity">Quantity</label>
            <input
              id="quantity"
              type="number"
              min="0"
              v-model.number="quantity"
              class="rounded-sm bg-white px-2 py-1 ring-1 dark:bg-gray-900"
            />

            <label for="file">Image</label>
            <div class="flex flex-wrap items-center gap-2">
              <input
                id="file"
                type="file"
                class="rounded-sm bg-white ring-1 dark:bg-gray-900 dark:hover:bg-gray-700"
                accept="image/*"
                multiple
                @change="
                  (e) => {
                    files = Array.from(
                      (e.target as HTMLInputElement).files || [],
                    );
                  }
                "
              />
              <span v-if="files.length > 0" class="text-sm text-gray-500">
                {{ files.length }} file(s) selected
              </span>
            </div>
            <!-- Buttons -->
            <div class="mt-8 flex gap-4">
              <button
                type="button"
                @click="handleSubmit"
                class="relative h-10 rounded-full bg-blue-500 px-4 py-2 text-white shadow hover:bg-blue-600"
              >
                Submit
                <KbdHint contents="Enter" :show="props.showHint" />
              </button>
              <button
                type="button"
                @click="handleDialogClose"
                class="relative h-10 rounded-full bg-red-500 px-4 py-2 text-white shadow hover:bg-red-600"
              >
                Cancel
                <KbdHint contents="Esc" :show="props.showHint" />
              </button>
              <button
                type="button"
                @click="handleCameraOpen"
                class="h-10 rounded-full bg-blue-500 px-4 py-2 text-white shadow hover:bg-blue-600"
              >
                Camera
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
