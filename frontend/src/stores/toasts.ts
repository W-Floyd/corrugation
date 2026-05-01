import { defineStore } from "pinia";
import { ref } from "vue";

export type ToastLevel = "success" | "info" | "warn" | "error";

export interface Toast {
  id: number;
  message: string;
  level: ToastLevel;
  persistent: boolean;
}

export const useToastsStore = defineStore("toasts", () => {
  const items = ref<Toast[]>([]);

  function add(message: string, level: ToastLevel = "error", persistent = false): number {
    const id = Date.now();
    items.value = [...items.value, { id, message, level, persistent }];
    if (!persistent) setTimeout(() => remove(id), 5000);
    return id;
  }

  function update(id: number, message: string, level?: ToastLevel): void {
    items.value = items.value.map((t) =>
      t.id === id ? { ...t, message, level: level ?? t.level } : t,
    );
  }

  // Clears persistent flag and starts the 5s auto-dismiss timer.
  function finalize(id: number): void {
    items.value = items.value.map((t) =>
      t.id === id ? { ...t, persistent: false } : t,
    );
    setTimeout(() => remove(id), 5000);
  }

  function remove(id: number): void {
    items.value = items.value.filter((t) => t.id !== id);
  }

  return { items, add, update, finalize, remove };
});
