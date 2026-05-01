<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useToast } from "primevue/usetoast";
import { api } from "@/api";
import { DEFAULT_TOAST_LIFE } from "@/stores/constants";

const router = useRouter();
const authStore = useAuthStore();
const toast = useToast();

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
    const [cfg, gcfg] = await Promise.all([
      api.getUserConfig(),
      api.getGlobalConfig(),
    ]);
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
      infinityTextDocumentPrefix:
        userConfig.value.infinityTextDocumentPrefix || null,
    });
    toast.add({
      severity: "success",
      summary: "Settings Saved",
      detail: "User settings saved",
      life: DEFAULT_TOAST_LIFE,
    });
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
    toast.add({
      severity: "success",
      summary: "Settings Saved",
      detail: "Global settings saved",
      life: DEFAULT_TOAST_LIFE,
    });
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

async function toggleAdmin(user: {
  id: number;
  username: string;
  isAdmin: boolean;
}) {
  const newValue = !user.isAdmin;
  try {
    await api.setUserAdmin(user.username, newValue);
    user.isAdmin = newValue;
    toast.add({
      severity: "success",
      summary: "Admin Status Updated",
      detail: `${user.username} is ${newValue ? "now an admin" : "no longer an admin"}`,
      life: DEFAULT_TOAST_LIFE,
    });
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
  <div
    class="min-h-screen bg-gray-50 text-gray-900 dark:bg-gray-900 dark:text-white"
  >
    <!-- Header -->
    <div class="container mx-auto px-4 pt-4">
      <div class="mb-6 flex items-center gap-4">
        <PrimeVueButton
          @click="router.push({ path: '/' })"
          link
          label="← Back"
        />
        <h1 class="pb-2 text-2xl font-semibold">Settings</h1>
        <span
          v-if="currentUsername"
          class="ml-auto flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400"
        >
          <span
            >Signed in as <strong>{{ currentUsername }}</strong></span
          >
          <span
            v-if="isAdmin"
            class="rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900 dark:text-blue-300"
            >admin</span
          >
        </span>
      </div>

      <!-- Tabs -->
      <div
        class="mb-8 flex gap-1 border-b border-gray-200 dark:border-gray-700"
      >
        <PrimeVueButton
          v-for="tab in isAdmin
            ? (['user', 'global', 'users'] as Tab[])
            : (['user'] as Tab[])"
          :key="tab"
          @click="selectTab(tab)"
          :class="[
            'rounded-t-lg px-4 py-2 text-sm font-medium transition-colors',
            activeTab === tab
              ? '-mb-px border border-b-white bg-white text-blue-600 dark:border-gray-700 dark:border-b-gray-800 dark:bg-gray-800 dark:text-sky-400'
              : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white',
          ]"
        >
          {{
            tab === "user"
              ? "My Settings"
              : tab === "global"
                ? "Global Settings"
                : "Users"
          }}
        </PrimeVueButton>
      </div>

      <!-- User Settings Tab -->
      <div v-if="activeTab === 'user'" class="max-w-lg pt-2">
        <div v-if="userConfigLoading" class="text-gray-500">Loading…</div>
        <form
          v-else
          @submit.prevent="saveUserConfig"
          class="flex flex-col gap-4"
        >
          <p class="text-sm text-gray-500 dark:text-gray-400">
            Override Infinity embedding model settings for your account. Leave
            blank to use server defaults.
          </p>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text embedding model</label
            >
            <input
              v-model="userConfig.infinityTextModel"
              type="text"
              :placeholder="globalConfig.infinityTextModel || 'Server default'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Image embedding model</label
            >
            <input
              v-model="userConfig.infinityImageModel"
              type="text"
              :placeholder="globalConfig.infinityImageModel || 'Server default'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text query prefix</label
            >
            <input
              v-model="userConfig.infinityTextQueryPrefix"
              type="text"
              :placeholder="
                globalConfig.infinityTextQueryPrefix || 'Server default'
              "
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text document prefix</label
            >
            <input
              v-model="userConfig.infinityTextDocumentPrefix"
              type="text"
              :placeholder="globalConfig.infinityTextDocumentPrefix || 'None'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <PrimeVueButton
              type="submit"
              :disabled="userConfigSaving"
              :label="userConfigSaving ? 'Saving…' : 'Save'"
            />
          </div>
        </form>
      </div>

      <!-- Global Settings Tab -->
      <div v-if="activeTab === 'global'" class="max-w-lg pt-2">
        <div v-if="globalConfigLoading" class="text-gray-500">Loading…</div>
        <form
          v-else
          @submit.prevent="saveGlobalConfig"
          class="flex flex-col gap-4"
        >
          <div>
            <label class="block pb-1 text-sm font-medium">Log level</label>
            <select
              v-model="globalConfig.logLevel"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            >
              <option
                v-for="lvl in ['silent', 'error', 'warn', 'info', 'debug']"
                :key="lvl"
                :value="lvl"
              >
                {{ lvl }}
              </option>
            </select>
          </div>

          <div class="flex items-center gap-3">
            <input
              v-model="globalConfig.backfillRecordEmbeddingsOnStart"
              id="backfillRecords"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="backfillRecords" class="text-sm font-medium"
              >Backfill record text embeddings on start</label
            >
          </div>

          <div class="flex items-center gap-3">
            <input
              v-model="globalConfig.backfillArtifactEmbeddingsOnStart"
              id="backfillArtifacts"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="backfillArtifacts" class="text-sm font-medium"
              >Backfill artifact image embeddings on start</label
            >
          </div>

          <div class="flex items-center gap-3">
            <input
              v-model="globalConfig.backfillArtifactOwnersOnStart"
              id="backfillOwners"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="backfillOwners" class="text-sm font-medium"
              >Assign artifact owners on start</label
            >
          </div>

          <div class="flex items-center gap-3">
            <input
              v-model="globalConfig.allowLocalUsernameLogin"
              id="localLogin"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="localLogin" class="text-sm font-medium"
              >Allow local username login (testing only)</label
            >
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Embedding model defaults (used when users have no override)
          </p>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text embedding model</label
            >
            <input
              v-model="globalConfig.infinityTextModel"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Image embedding model</label
            >
            <input
              v-model="globalConfig.infinityImageModel"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text query prefix</label
            >
            <input
              v-model="globalConfig.infinityTextQueryPrefix"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="block pb-1 text-sm font-medium"
              >Text document prefix</label
            >
            <input
              v-model="globalConfig.infinityTextDocumentPrefix"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <PrimeVueButton
              type="submit"
              :disabled="globalConfigSaving"
              :label="globalConfigSaving ? 'Saving…' : 'Save'"
            />
          </div>
        </form>
      </div>

      <!-- Users Tab -->
      <div v-if="activeTab === 'users'" class="max-w-lg pt-2">
        <div v-if="usersLoading" class="text-gray-500">Loading…</div>
        <div v-else>
          <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
            Manage admin access for all users.
          </p>
          <div class="flex flex-col gap-2">
            <div
              v-for="user in users"
              :key="user.id"
              class="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800"
            >
              <div class="flex items-center gap-3">
                <span class="text-sm font-medium">{{ user.username }}</span>
                <span
                  v-if="user.username === currentUsername"
                  class="text-xs text-gray-400 dark:text-gray-500"
                  >(you)</span
                >
              </div>
              <div class="flex items-center gap-3">
                <span
                  v-if="user.isAdmin"
                  class="rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900 dark:text-blue-300"
                  >admin</span
                >
                <PrimeVueButton
                  @click="toggleAdmin(user)"
                  :disabled="user.isAdmin && user.username === currentUsername"
                  :title="
                    user.isAdmin && user.username === currentUsername
                      ? 'Cannot remove admin from yourself'
                      : undefined
                  "
                  :label="user.isAdmin ? 'Remove admin' : 'Make admin'"
                  :severity="user.isAdmin ? 'danger' : 'secondary'"
                  size="small"
                />
              </div>
            </div>
            <div v-if="users.length === 0" class="text-sm text-gray-500">
              No users found.
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
