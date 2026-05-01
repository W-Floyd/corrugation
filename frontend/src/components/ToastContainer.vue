<script setup lang="ts" name="ToastContainer">
import { ref, watch } from "vue";
import { useToastsStore } from "@/stores/toasts";
import type { ToastLevel } from "@/stores/toasts";

const toastsStore = useToastsStore();
const visibleToasts = ref(new Map<number, boolean>());

// Watch for new toasts and mark them visible immediately
watch(
  () => toastsStore.items,
  (items) => {
    for (const toast of items) {
      if (!visibleToasts.value.has(toast.id)) {
        visibleToasts.value.set(toast.id, true);
      }
    }
  },
  { immediate: true, deep: true },
);

const hideToast = (id: number): void => {
  visibleToasts.value.set(id, false);
  setTimeout(() => {
    toastsStore.remove(id);
    visibleToasts.value.delete(id);
  }, 300);
};

const levelClasses: Record<ToastLevel, string> = {
  error:
    "bg-red-50 dark:bg-red-900/40 text-red-700 dark:text-red-300 ring-red-200 dark:ring-red-700",
  warn: "bg-amber-50 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 ring-amber-200 dark:ring-amber-700",
  info: "bg-blue-50 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 ring-blue-200 dark:ring-blue-700",
  success:
    "bg-green-50 dark:bg-green-900/40 text-green-700 dark:text-green-300 ring-green-200 dark:ring-green-700",
};

const dismissClasses: Record<ToastLevel, string> = {
  error: "text-red-400 hover:text-red-600 dark:hover:text-red-200",
  warn: "text-amber-400 hover:text-amber-600 dark:hover:text-amber-200",
  info: "text-blue-400 hover:text-blue-600 dark:hover:text-blue-200",
  success: "text-green-400 hover:text-green-600 dark:hover:text-green-200",
};
</script>

<template>
  <div
    class="pointer-events-none fixed right-4 bottom-4 z-50 flex flex-col items-end gap-2"
  >
    <TransitionGroup
      name="toast"
      tag="div"
      class="flex flex-col items-end gap-2"
    >
      <div
        v-for="toast in toastsStore.items"
        :key="toast.id"
        v-show="visibleToasts.get(toast.id)"
        @mouseenter="visibleToasts.set(toast.id, true)"
        @mouseleave="hideToast(toast.id)"
        :class="[
          'pointer-events-auto flex max-w-sm items-start gap-2 rounded-lg px-4 py-3 shadow-lg ring-1',
          levelClasses[toast.level],
        ]"
      >
        <span class="wrap-break-words text-sm">{{ toast.message }}</span>
        <button
          type="button"
          @click="hideToast(toast.id)"
          :class="['shrink-0', dismissClasses[toast.level]]"
          title="Dismiss"
        >
          &times;
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 0.3s ease-out,
    transform 0.3s ease-out;
}

.toast-enter-from {
  opacity: 0;
  transform: translateY(0.5rem);
}

.toast-leave-to {
  opacity: 0;
  transform: translateY(0.5rem);
}
</style>
