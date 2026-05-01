<script setup lang="ts">
import { ref, watchEffect, watch, nextTick, onUnmounted } from "vue";
import { useCameraStore } from "@/stores/camera";
import RefreshIcon from "vue-material-design-icons/Refresh.vue";

const cameraStore = useCameraStore();
const videoEl = ref<HTMLVideoElement | null>(null);
const lastDeviceId = ref<string | null>(null);

const handleKeydown = (e: KeyboardEvent): void => {
  if (cameraStore.previewUrl) {
    if (e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.confirm();
    } else if (e.key === "r" || e.key === "R") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.rotate();
    } else if (e.key === "c" || e.key === "C") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.retake();
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.close();
    }
  } else {
    if (e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.capture();
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      cameraStore.close();
    }
  }
};

watch(
  () => cameraStore.opened,
  async (val) => {
    if (val) {
      window.addEventListener("keydown", handleKeydown);
      await cameraStore.loadDevices();
      // Set default device if none selected
      if (!cameraStore.selectedDeviceId && cameraStore.devices.length > 0) {
        const firstDevice = cameraStore.devices[0];
        if (firstDevice && firstDevice.deviceId) {
          cameraStore.selectedDeviceId = firstDevice.deviceId;
        }
      }
    } else {
      window.removeEventListener("keydown", handleKeydown);
    }
  },
);

watch(
  () => cameraStore.selectedDeviceId,
  async (newId) => {
    if (
      newId &&
      newId !== lastDeviceId.value &&
      cameraStore.opened &&
      cameraStore.stream &&
      cameraStore.callback
    ) {
      lastDeviceId.value = newId;
      // Switch to new device directly without closing
      // Stop all tracks from the old stream
      cameraStore.stream?.getTracks().forEach((track) => track.stop());
      // Request new stream immediately with new device
      const constraints = {
        deviceId: newId,
        width: { ideal: 1920 },
        height: { ideal: 1080 },
        aspectRatio: { ideal: 16 / 9 },
      };
      navigator.mediaDevices
        .getUserMedia({
          video: constraints,
          audio: false,
        })
        .then((newStream) => {
          cameraStore.stream = newStream;
        })
        .catch((e) => {
          console.error("Failed to switch camera:", e);
        });
    }
  },
);

onUnmounted(() => window.removeEventListener("keydown", handleKeydown));

watchEffect(async () => {
  if (cameraStore.opened && cameraStore.stream) {
    await nextTick();
    if (videoEl.value) {
      videoEl.value.srcObject = cameraStore.stream;
    }
  }
});
</script>

<template>
  <Teleport to="body">
    <div
      v-show="cameraStore.opened"
      v-if="cameraStore.opened"
      class="fixed inset-0 z-50 bg-black"
    >
      <!-- Live viewfinder -->
      <video
        v-show="!cameraStore.previewUrl"
        ref="videoEl"
        id="cameraVideo"
        autoplay
        playsinline
        class="absolute inset-0 h-full w-full object-contain"
      ></video>

      <!-- Preview after capture -->
      <img
        v-show="cameraStore.previewUrl"
        :src="cameraStore.previewUrl ?? undefined"
        class="absolute inset-0 h-full w-full object-contain"
      />

      <canvas id="cameraCanvas" class="hidden"></canvas>

      <!-- Camera selector -->
      <div
        v-if="cameraStore.devices.length > 0 && !cameraStore.previewUrl"
        class="absolute top-4 left-0 z-10 flex w-full justify-center gap-2 px-4"
      >
        <select
          v-model="cameraStore.selectedDeviceId"
          class="h-10 rounded-full bg-gray-800 px-4 py-2 text-white shadow-lg ring-1 ring-gray-600"
        >
          <option
            v-for="device in cameraStore.devices"
            :key="device.deviceId"
            :value="device.deviceId"
          >
            {{ device.label || `Camera ${device.deviceId.slice(0, 8)}...` }}
          </option>
        </select>
        <PrimeVueButton
          @click="cameraStore.loadDevices()"
          rounded
          class="h-10 w-10 p-0"
          title="Reload cameras"
          aria-label="Reload cameras"
        >
          <RefreshIcon :size="20" />
        </PrimeVueButton>
      </div>

      <!-- Shooting controls -->
      <div
        v-show="!cameraStore.previewUrl"
        class="absolute bottom-0 left-0 flex w-full flex-row items-center justify-center gap-4"
        style="padding-bottom: max(2rem, env(safe-area-inset-bottom))"
      >
        <PrimeVueButton
          @click="cameraStore.capture()"
          rounded
          severity="secondary"
          class="h-16 w-16 border-4 border-gray-300 p-0 active:scale-95"
          title="Capture photo"
        />
        <PrimeVueButton
          @click="cameraStore.close()"
          rounded
          severity="danger"
          label="Cancel"
          style="transition: transform 0.3s ease"
        />
      </div>

      <!-- Preview controls -->
      <div
        v-show="cameraStore.previewUrl"
        class="absolute bottom-0 left-0 flex w-full flex-row items-center justify-center gap-4"
        style="padding-bottom: max(2rem, env(safe-area-inset-bottom))"
      >
        <PrimeVueButton @click="cameraStore.confirm()" rounded label="Use" />
        <PrimeVueButton
          @click="cameraStore.rotate()"
          rounded
          severity="warn"
          label="Rotate"
        />
        <PrimeVueButton
          @click="cameraStore.retake()"
          rounded
          severity="secondary"
          label="Retake"
        />
        <PrimeVueButton
          @click="cameraStore.close()"
          rounded
          severity="danger"
          label="Cancel"
        />
      </div>
    </div>
  </Teleport>
</template>
