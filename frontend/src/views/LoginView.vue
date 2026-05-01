<template>
  <div
    class="flex min-h-screen items-center justify-center bg-gray-100 dark:bg-gray-900"
  >
    <div
      class="flex w-full max-w-sm flex-col gap-6 rounded-2xl bg-white p-8 shadow-lg dark:bg-gray-800"
    >
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Sign in</h1>

      <!-- Local username login -->
      <div class="space-y-4">
        <input
          v-model="username"
          type="text"
          placeholder="Username"
          class="w-full rounded-lg border border-gray-300 bg-white px-4 py-2 text-gray-900 focus:ring-2 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          @keydown.enter="handleLocalLogin"
        />
        <PrimeVueButton
          @click="handleLocalLogin"
          label="Sign in with username"
          class="w-full"
          :disabled="!username.trim()"
        />
      </div>

      <!-- OIDC login -->
      <PrimeVueButton
        v-if="authStore.authConfig.enabled"
        @click="authStore.startLogin()"
        label="Sign in with Authentik"
        class="w-full"
        title="Sign in with Authentik"
      />

      <p
        v-if="authStore.authConfig.enabled"
        class="text-center text-sm text-gray-500 dark:text-gray-400"
      >
        OIDC authentication enabled
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useToast } from "primevue/usetoast";
import { DEFAULT_TOAST_LIFE } from "@/stores/constants";

const router = useRouter();
const authStore = useAuthStore();
const username = ref("");

async function handleLocalLogin(): Promise<void> {
  if (!username.value.trim()) return;
  try {
    await authStore.localLogin(username.value.trim());
    const toast = useToast();
    toast.add({
      severity: "success",
      summary: "Logged In",
      detail: `Logged in as ${username.value.trim()}`,
      life: DEFAULT_TOAST_LIFE,
    });
    router.push({ path: "/" });
  } catch {
    // error toast already added by the store
  }
}
</script>
