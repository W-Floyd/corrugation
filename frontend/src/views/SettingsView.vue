<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";

const router = useRouter();
const authStore = useAuthStore();
const toastsStore = useToastsStore();

type Tab = "user" | "global" | "users";
const activeTab = ref<Tab>("user");

// --- User config ---
const userConfig = ref({
  infinityTextModel: "" as string,
  infinityImageModel: "" as string,
  infinityTextQueryPrefix: "" as string,
  infinityTextDocumentPrefix: "" as string,
});
const userConfigLoading = ref(false);
const userConfigSaving = ref(false);

// --- Global config ---
const globalConfig = ref({
  logLevel: "warn",
  backfillRecordEmbeddingsOnStart: false,
  backfillArtifactEmbeddingsOnStart: false,
  backfillArtifactOwnersOnStart: false,
  allowLocalUsernameLogin: false,
  infinityTextModel: "",
  infinityImageModel: "",
  infinityTextQueryPrefix: "",
  infinityTextDocumentPrefix: "",
});
const globalConfigLoading = ref(false);
const globalConfigSaving = ref(false);

// --- Users list ---
const users = ref<{ id: number; username: string; isAdmin: boolean }[]>([]);
const usersLoading = ref(false);

const isAdmin = computed(() => authStore.isAdmin);
const currentUsername = computed(() => authStore.username);

async function loadUserConfig() {
  userConfigLoading.value = true;
  try {
    const [cfg, gcfg] = await Promise.all([api.getUserConfig(), api.getGlobalConfig()]);
    globalConfig.value = {
      ...globalConfig.value,
      infinityTextModel: gcfg.infinityTextModel,
      infinityImageModel: gcfg.infinityImageModel,
      infinityTextQueryPrefix: gcfg.infinityTextQueryPrefix,
      infinityTextDocumentPrefix: gcfg.infinityTextDocumentPrefix,
    };
    userConfig.value = {
      infinityTextModel: cfg.infinityTextModel ?? "",
      infinityImageModel: cfg.infinityImageModel ?? "",
      infinityTextQueryPrefix: cfg.infinityTextQueryPrefix ?? "",
      infinityTextDocumentPrefix: cfg.infinityTextDocumentPrefix ?? "",
    };
  } catch {
    // toast already shown by apiFetch
  } finally {
    userConfigLoading.value = false;
  }
}

async function saveUserConfig() {
  userConfigSaving.value = true;
  try {
    await api.updateUserConfig({
      infinityTextModel: userConfig.value.infinityTextModel || null,
      infinityImageModel: userConfig.value.infinityImageModel || null,
      infinityTextQueryPrefix: userConfig.value.infinityTextQueryPrefix || null,
      infinityTextDocumentPrefix: userConfig.value.infinityTextDocumentPrefix || null,
    });
    toastsStore.add("User settings saved", "success");
  } catch {
    // toast already shown
  } finally {
    userConfigSaving.value = false;
  }
}

async function loadGlobalConfig() {
  globalConfigLoading.value = true;
  try {
    const cfg = await api.getGlobalConfig();
    globalConfig.value = {
      logLevel: cfg.logLevel,
      backfillRecordEmbeddingsOnStart: cfg.backfillRecordEmbeddingsOnStart,
      backfillArtifactEmbeddingsOnStart: cfg.backfillArtifactEmbeddingsOnStart,
      backfillArtifactOwnersOnStart: cfg.backfillArtifactOwnersOnStart,
      allowLocalUsernameLogin: cfg.allowLocalUsernameLogin,
      infinityTextModel: cfg.infinityTextModel,
      infinityImageModel: cfg.infinityImageModel,
      infinityTextQueryPrefix: cfg.infinityTextQueryPrefix,
      infinityTextDocumentPrefix: cfg.infinityTextDocumentPrefix,
    };
  } catch {
    // toast already shown
  } finally {
    globalConfigLoading.value = false;
  }
}

async function saveGlobalConfig() {
  globalConfigSaving.value = true;
  try {
    await api.updateGlobalConfig(globalConfig.value);
    toastsStore.add("Global settings saved", "success");
  } catch {
    // toast already shown
  } finally {
    globalConfigSaving.value = false;
  }
}

async function loadUsers() {
  usersLoading.value = true;
  try {
    users.value = await api.getUsers();
  } catch {
    // toast already shown
  } finally {
    usersLoading.value = false;
  }
}

async function toggleAdmin(user: { id: number; username: string; isAdmin: boolean }) {
  const newValue = !user.isAdmin;
  try {
    await api.setUserAdmin(user.username, newValue);
    user.isAdmin = newValue;
    toastsStore.add(
      `${user.username} is ${newValue ? "now an admin" : "no longer an admin"}`,
      "success",
    );
  } catch {
    // toast already shown
  }
}

function selectTab(tab: Tab) {
  activeTab.value = tab;
  if (tab === "user") loadUserConfig();
  else if (tab === "global") loadGlobalConfig();
  else if (tab === "users") loadUsers();
}

onMounted(() => {
  loadUserConfig();
});
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-white">
    <!-- Header -->
    <div class="container mx-auto pt-4 px-4">
      <div class="flex items-center gap-4 mb-6">
        <button @click="router.push({ path: '/' })"
          class="text-blue-600 dark:text-sky-400 hover:underline text-sm">
          ← Back
        </button>
        <h1 class="text-2xl font-semibold">Settings</h1>
        <span v-if="currentUsername" class="text-sm text-gray-500 dark:text-gray-400 ml-auto">
          Signed in as <strong>{{ currentUsername }}</strong>
          <span v-if="isAdmin" class="ml-2 text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded-full">admin</span>
        </span>
      </div>

      <!-- Tabs -->
      <div class="flex gap-1 border-b border-gray-200 dark:border-gray-700 mb-6">
        <button v-for="tab in (isAdmin ? ['user', 'global', 'users'] as Tab[] : ['user'] as Tab[])"
          :key="tab"
          @click="selectTab(tab)"
          :class="[
            'px-4 py-2 text-sm font-medium rounded-t-lg transition-colors',
            activeTab === tab
              ? 'bg-white dark:bg-gray-800 border border-b-white dark:border-gray-700 dark:border-b-gray-800 -mb-px text-blue-600 dark:text-sky-400'
              : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white',
          ]">
          {{ tab === "user" ? "My Settings" : tab === "global" ? "Global Settings" : "Users" }}
        </button>
      </div>

      <!-- User Settings Tab -->
      <div v-if="activeTab === 'user'" class="max-w-lg">
        <div v-if="userConfigLoading" class="text-gray-500">Loading…</div>
        <form v-else @submit.prevent="saveUserConfig" class="flex flex-col gap-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">
            Override Infinity embedding model settings for your account. Leave blank to use server defaults.
          </p>

          <div>
            <label class="block text-sm font-medium mb-1">Text embedding model</label>
            <input v-model="userConfig.infinityTextModel" type="text" :placeholder="globalConfig.infinityTextModel || 'Server default'"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Image embedding model</label>
            <input v-model="userConfig.infinityImageModel" type="text" :placeholder="globalConfig.infinityImageModel || 'Server default'"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Text query prefix</label>
            <input v-model="userConfig.infinityTextQueryPrefix" type="text" :placeholder="globalConfig.infinityTextQueryPrefix || 'Server default'"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Text document prefix</label>
            <input v-model="userConfig.infinityTextDocumentPrefix" type="text" :placeholder="globalConfig.infinityTextDocumentPrefix || 'None'"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <button type="submit" :disabled="userConfigSaving"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
              {{ userConfigSaving ? "Saving…" : "Save" }}
            </button>
          </div>
        </form>
      </div>

      <!-- Global Settings Tab -->
      <div v-if="activeTab === 'global'" class="max-w-lg">
        <div v-if="globalConfigLoading" class="text-gray-500">Loading…</div>
        <form v-else @submit.prevent="saveGlobalConfig" class="flex flex-col gap-4">
          <div>
            <label class="block text-sm font-medium mb-1">Log level</label>
            <select v-model="globalConfig.logLevel"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm">
              <option v-for="lvl in ['silent', 'error', 'warn', 'info', 'debug']" :key="lvl" :value="lvl">{{ lvl }}</option>
            </select>
          </div>

          <div class="flex items-center gap-3">
            <input v-model="globalConfig.backfillRecordEmbeddingsOnStart" id="backfillRecords" type="checkbox"
              class="w-4 h-4 rounded border-gray-300 text-blue-600" />
            <label for="backfillRecords" class="text-sm font-medium">Backfill record text embeddings on start</label>
          </div>

          <div class="flex items-center gap-3">
            <input v-model="globalConfig.backfillArtifactEmbeddingsOnStart" id="backfillArtifacts" type="checkbox"
              class="w-4 h-4 rounded border-gray-300 text-blue-600" />
            <label for="backfillArtifacts" class="text-sm font-medium">Backfill artifact image embeddings on start</label>
          </div>

          <div class="flex items-center gap-3">
            <input v-model="globalConfig.backfillArtifactOwnersOnStart" id="backfillOwners" type="checkbox"
              class="w-4 h-4 rounded border-gray-300 text-blue-600" />
            <label for="backfillOwners" class="text-sm font-medium">Assign artifact owners on start</label>
          </div>

          <div class="flex items-center gap-3">
            <input v-model="globalConfig.allowLocalUsernameLogin" id="localLogin" type="checkbox"
              class="w-4 h-4 rounded border-gray-300 text-blue-600" />
            <label for="localLogin" class="text-sm font-medium">Allow local username login (testing only)</label>
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">Embedding model defaults (used when users have no override)</p>

          <div>
            <label class="block text-sm font-medium mb-1">Text embedding model</label>
            <input v-model="globalConfig.infinityTextModel" type="text"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Image embedding model</label>
            <input v-model="globalConfig.infinityImageModel" type="text"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Text query prefix</label>
            <input v-model="globalConfig.infinityTextQueryPrefix" type="text"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <label class="block text-sm font-medium mb-1">Text document prefix</label>
            <input v-model="globalConfig.infinityTextDocumentPrefix" type="text"
              class="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm" />
          </div>

          <div>
            <button type="submit" :disabled="globalConfigSaving"
              class="px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
              {{ globalConfigSaving ? "Saving…" : "Save" }}
            </button>
          </div>
        </form>
      </div>

      <!-- Users Tab -->
      <div v-if="activeTab === 'users'" class="max-w-lg">
        <div v-if="usersLoading" class="text-gray-500">Loading…</div>
        <div v-else>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
            Manage admin access for all users.
          </p>
          <div class="flex flex-col gap-2">
            <div v-for="user in users" :key="user.id"
              class="flex items-center justify-between px-4 py-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
              <div class="flex items-center gap-3">
                <span class="text-sm font-medium">{{ user.username }}</span>
                <span v-if="user.username === currentUsername"
                  class="text-xs text-gray-400 dark:text-gray-500">(you)</span>
              </div>
              <div class="flex items-center gap-3">
                <span v-if="user.isAdmin"
                  class="text-xs bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded-full">admin</span>
                <button @click="toggleAdmin(user)"
                  :disabled="user.isAdmin && user.username === currentUsername"
                  :title="user.isAdmin && user.username === currentUsername ? 'Cannot remove admin from yourself' : undefined"
                  :class="[
                    'text-xs px-3 py-1 rounded-lg font-medium transition-colors',
                    user.isAdmin && user.username === currentUsername
                      ? 'opacity-40 cursor-not-allowed bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-400'
                      : user.isAdmin
                        ? 'bg-red-100 dark:bg-red-900/40 text-red-700 dark:text-red-400 hover:bg-red-200 dark:hover:bg-red-900'
                        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600',
                  ]">
                  {{ user.isAdmin ? "Remove admin" : "Make admin" }}
                </button>
              </div>
            </div>
            <div v-if="users.length === 0" class="text-gray-500 text-sm">No users found.</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
