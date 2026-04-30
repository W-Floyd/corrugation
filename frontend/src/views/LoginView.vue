<template>
    <div class="min-h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900">
        <div class="w-full max-w-sm bg-white dark:bg-gray-800 rounded-2xl shadow-lg p-8 flex flex-col gap-6">
            <h1 class="text-2xl font-bold text-gray-900 dark:text-white">Sign in</h1>

            <!-- Local username login -->
            <div class="space-y-4">
                <input v-model="username" type="text" placeholder="Username"
                    class="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500"
                    @keydown.enter="handleLocalLogin" />
                <button @click="handleLocalLogin"
                    class="w-full rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 text-sm transition-colors"
                    :disabled="!username.trim()">
                    Sign in with username
                </button>
            </div>

            <!-- OIDC login -->
            <button v-if="authStore.authConfig.enabled" @click="authStore.startLogin()"
                class="rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 text-sm transition-colors w-full"
                title="Sign in with Authentik">
                Sign in with Authentik
            </button>

            <p v-if="authStore.authConfig.enabled" class="text-sm text-gray-500 dark:text-gray-400 text-center">
                OIDC authentication enabled
            </p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useToastsStore } from "../stores/toasts";

const router = useRouter();
const authStore = useAuthStore();
const toastsStore = useToastsStore();
const username = ref("");

async function handleLocalLogin(): Promise<void> {
    if (!username.value.trim()) return;
    try {
        await authStore.localLogin(username.value.trim());
        toastsStore.add(`Logged in as ${username.value.trim()}`, "success");
        router.push({ name: "entity" });
    } catch {
        // error toast already added by the store
    }
}
</script>
