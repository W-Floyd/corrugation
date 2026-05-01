<script setup lang="ts" name="QuickCaptureCard">
import CameraIcon from "vue-material-design-icons/Camera.vue";
import { useCameraStore } from "@/stores/camera";
import { useRecordsStore } from "@/stores/records";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";
import { appRecordToRecordBody } from "@/api/types";

const recordsStore = useRecordsStore();
const cameraStore = useCameraStore();
const toastsStore = useToastsStore();

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

                    await api.createRecord(appRecordToRecordBody({
                        id: 0,
                        name: null,
                        description: null,
                        artifacts: [artifactId],
                        location: recordId,
                        metadata: {
                            quantity: null,
                            owner: null,
                            referenceNumber: null,
                            lastModified: null,
                        },
                    }));
                    await recordsStore.reload();
                    toastsStore.add("Record created from photo", "success");
                } catch (error) {
                    console.error("Failed to create record:", error);
                    toastsStore.add("Failed to create record from photo");
                }

                resolve();
            });
        });
    } catch (error) {
        console.error("Camera error:", error);
        toastsStore.add("Camera error");
    }
};
</script>

<template>
    <figure
        class="container relative h-full max-w-sm min-h-40 grow flex items-center justify-center rounded-xl border-2 border-dashed border-gray-300 dark:border-gray-600 bg-transparent cursor-pointer hover:border-blue-400 dark:hover:border-blue-500 hover:bg-blue-50/50 dark:hover:bg-blue-900/10 transition-colors"
        @click="handleQuickCapture(recordsStore.currentRecord)">
        <div
            class="flex flex-col items-center gap-2 text-gray-400 dark:text-gray-500 hover:text-blue-400 dark:hover:text-blue-500 pointer-events-none select-none">
            <CameraIcon :size="40" />
            <span class="text-sm">Tap to capture</span>
        </div>
    </figure>
</template>
