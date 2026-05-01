<script setup lang="ts" name="QuickCaptureCard">
import CameraIcon from "vue-material-design-icons/Camera.vue";
import { useCameraStore } from "@/stores/camera";
import { useRecordsStore } from "@/stores/records";
import { useToast } from "@/utils/toast";
import { api } from "@/api";
import { DEFAULT_TOAST_LIFE } from "@/stores/constants";

const recordsStore = useRecordsStore();
const cameraStore = useCameraStore();
const toast = useToast();

const handleQuickCapture = async (recordId: number): Promise<void> => {
  try {
    await new Promise<void>((resolve) => {
      cameraStore.open(async (files: File[]) => {
        if (files.length === 0 || !files[0]) {
          resolve();
          return;
        }

        try {
          // Upload artifact
          const artifactId = await api.uploadArtifact(files[0]);

          await api.createRecord({
            Title: null,
            ReferenceNumber: null,
            Description: null,
            Quantity: null,
            ParentID: recordId,
            Artifacts: [artifactId],
          });
          await recordsStore.reload();
          toast.add({
            severity: "success",
            summary: "Record Created",
            detail: "Record created from photo",
            life: DEFAULT_TOAST_LIFE,
          });
        } catch (error) {
          console.error("Failed to create record:", error);
          toast.add({
            severity: "error",
            summary: "Failed to Create Record",
            detail: "Failed to create record from photo",
            life: DEFAULT_TOAST_LIFE,
          });
        }

        resolve();
      });
    });
  } catch (error) {
    console.error("Camera error:", error);
    toast.add({
      severity: "error",
      summary: "Camera Error",
      detail: "Camera error occurred",
      life: DEFAULT_TOAST_LIFE,
    });
  }
};
</script>

<template>
  <figure
    class="relative container flex h-full min-h-40 max-w-sm grow cursor-pointer items-center justify-center rounded-xl border-2 border-dashed border-gray-300 bg-transparent transition-colors hover:border-blue-400 hover:bg-blue-50/50 dark:border-gray-600 dark:hover:border-blue-500 dark:hover:bg-blue-900/10"
    @click="handleQuickCapture(recordsStore.currentRecord)"
  >
    <div
      class="pointer-events-none flex flex-col items-center gap-2 text-gray-400 select-none hover:text-blue-400 dark:text-gray-500 dark:hover:text-blue-500"
    >
      <CameraIcon :size="40" />
      <span class="text-sm">Tap to capture</span>
    </div>
  </figure>
</template>
