import { useToast as usePrimeToast } from "primevue/usetoast";
import type { ToastMessageOptions } from "primevue/toast";

export function useToast() {
  const prime = usePrimeToast();
  return {
    add(options: ToastMessageOptions) {
      const detail = options.detail ? ` — ${options.detail}` : "";
      console.log(`[toast:${options.severity}] ${options.summary}${detail}`);
      prime.add(options);
    },
  };
}
