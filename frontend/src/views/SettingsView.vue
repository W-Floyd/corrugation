<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useToastsStore } from "@/stores/toasts";
import { useRecordsStore } from "@/stores/records";
import { api } from "@/api";

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const toastsStore = useToastsStore();

type Tab = "user" | "global" | "users" | "jobs" | "suggestion-jobs";
const validTabs: Tab[] = ["user", "global", "users", "jobs", "suggestion-jobs"];

function tabFromRoute(): Tab {
  const t = route.params.tab;
  const s = Array.isArray(t) ? t[0] : t;
  return validTabs.includes(s as Tab) ? (s as Tab) : "user";
}

const activeTab = ref<Tab>(tabFromRoute());

// --- User config ---
const userConfig = ref({
  infinityTextModel: "" as string,
  infinityImageModel: "" as string,
  infinityTextQueryPrefix: "" as string,
  infinityTextDocumentPrefix: "" as string,
  // null = use global default; string[] = explicit override (may be empty to disable)
  enabledBarcodeFormats: null as string[] | null,
  // null = inherit global; positive number = cap dimensions for this user
  maximumEmbeddingDimensions: null as number | null,
});
const userConfigLoading = ref(false);
const userConfigSaving = ref(false);
const invalidatingEmbeddings = ref(false);

// --- Global config ---
const globalConfig = ref({
  logLevel: "warn",
  backfillLegacyEmbeddingsOnStart: false,
  backfillRecordEmbeddingsOnStart: false,
  backfillArtifactEmbeddingsOnStart: false,
  backfillArtifactOwnersOnStart: false,
  backfillSuggestionsOnStart: false,
  allowLocalUsernameLogin: false,
  infinityTextModel: "",
  infinityImageModel: "",
  infinityTextQueryPrefix: "",
  infinityTextDocumentPrefix: "",
  enabledBarcodeFormats: [] as string[],
  // null = use model output as-is; positive number = cap embedding dimensions
  maximumEmbeddingDimensions: null as number | null,
  ollamaAddress: "",
  ollamaVisionModel: "",
});

const allBarcodeFormats = ref<{ value: string; label: string }[]>([]);

async function loadCapabilities() {
  try {
    const caps = await api.getCapabilities();
    allBarcodeFormats.value = caps.barcodeFormats;
  } catch {
    // non-fatal
  }
}

function isBarcodeFormatEnabled(fmt: string): boolean {
  return globalConfig.value.enabledBarcodeFormats.includes(fmt);
}

function toggleBarcodeFormat(fmt: string) {
  const idx = globalConfig.value.enabledBarcodeFormats.indexOf(fmt);
  if (idx === -1) {
    globalConfig.value.enabledBarcodeFormats.push(fmt);
  } else {
    globalConfig.value.enabledBarcodeFormats.splice(idx, 1);
  }
}
const globalConfigLoading = ref(false);
const globalConfigSaving = ref(false);
const recordsStore = useRecordsStore();
const ollamaModels = ref<string[]>([]);
const ollamaPullModel = ref("");
const ollamaPulling = ref(false);

async function loadOllamaModels() {
  ollamaModels.value = await api.getOllamaModels();
}

async function pullOllamaModel() {
  const model = ollamaPullModel.value.trim();
  if (!model) return;
  ollamaPulling.value = true;
  try {
    await api.pullOllamaModel(model); // returns 202 immediately
  } catch {
    ollamaPulling.value = false;
    // error toast already shown by apiFetch
  }
}

watch(
  () => recordsStore.lastOllamaPullEvent,
  async (event) => {
    if (!event) return;
    ollamaPulling.value = false;
    if (event.success) {
      toastsStore.add(`Pulled ${event.model}`, "success");
      await loadOllamaModels();
    } else {
      toastsStore.add(`Failed to pull ${event.model}`, "warn");
    }
  },
);

// --- Backfill ---
const backfillPreview = ref<{
  legacyEmbeddings: number;
  records: number;
  artifacts: number;
  suggestions: number;
} | null>(null);
const backfillPreviewLoading = ref(false);
const runningLegacyEmbeddingsBackfill = ref(false);
const runningRecordBackfill = ref(false);
const runningArtifactBackfill = ref(false);
const runningSuggestionsBackfill = ref(false);

// --- Users list ---
const users = ref<{ id: number; username: string; isAdmin: boolean }[]>([]);
const usersLoading = ref(false);

// --- Embedding jobs ---
type EmbeddingJob = {
  id: number;
  jobType: string;
  targetID: number;
  username: string;
  status: string;
  errorMsg?: string;
  retryCount: number;
  embedModel: string;
  dimensions?: number;
  source: string;
  createdAt: string;
  updatedAt: string;
};
const jobs = ref<EmbeddingJob[]>([]);
const jobsTotal = ref(0);
const jobsPage = ref(0);
const jobsPageSize = 50;
const jobsLoading = ref(false);
const jobsPageInput = ref(1);
const jobsShowAll = ref(false);
const jobsStatusFilter = ref("");

// --- Suggestion jobs ---
type SuggestionJob = {
  id: number;
  artifactID: number;
  ollamaModel: string;
  username: string;
  status: string;
  errorMsg?: string;
  retryCount: number;
  source: string;
  createdAt: string;
  updatedAt: string;
};
const suggestionJobs = ref<SuggestionJob[]>([]);
const suggestionJobsTotal = ref(0);
const suggestionJobsPage = ref(0);
const suggestionJobsPageSize = 50;
const suggestionJobsLoading = ref(false);
const suggestionJobsPageInput = ref(1);
const suggestionJobsShowAll = ref(false);
const suggestionJobsStatusFilter = ref("");

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
      enabledBarcodeFormats: gcfg.enabledBarcodeFormats ?? [],
      maximumEmbeddingDimensions: gcfg.maximumEmbeddingDimensions ?? null,
    };
    userConfig.value = {
      infinityTextModel: cfg.infinityTextModel ?? "",
      infinityImageModel: cfg.infinityImageModel ?? "",
      infinityTextQueryPrefix: cfg.infinityTextQueryPrefix ?? "",
      infinityTextDocumentPrefix: cfg.infinityTextDocumentPrefix ?? "",
      enabledBarcodeFormats: cfg.enabledBarcodeFormats ?? null,
      maximumEmbeddingDimensions: cfg.maximumEmbeddingDimensions ?? null,
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
      enabledBarcodeFormats: userConfig.value.enabledBarcodeFormats,
      maximumEmbeddingDimensions: userConfig.value.maximumEmbeddingDimensions,
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
      backfillLegacyEmbeddingsOnStart: cfg.backfillLegacyEmbeddingsOnStart,
      backfillRecordEmbeddingsOnStart: cfg.backfillRecordEmbeddingsOnStart,
      backfillArtifactEmbeddingsOnStart: cfg.backfillArtifactEmbeddingsOnStart,
      backfillArtifactOwnersOnStart: cfg.backfillArtifactOwnersOnStart,
      backfillSuggestionsOnStart: cfg.backfillSuggestionsOnStart ?? false,
      allowLocalUsernameLogin: cfg.allowLocalUsernameLogin,
      infinityTextModel: cfg.infinityTextModel,
      infinityImageModel: cfg.infinityImageModel,
      infinityTextQueryPrefix: cfg.infinityTextQueryPrefix,
      infinityTextDocumentPrefix: cfg.infinityTextDocumentPrefix,
      enabledBarcodeFormats: cfg.enabledBarcodeFormats ?? [],
      maximumEmbeddingDimensions: cfg.maximumEmbeddingDimensions ?? null,
      ollamaAddress: cfg.ollamaAddress ?? "",
      ollamaVisionModel: cfg.ollamaVisionModel ?? "",
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

async function loadBackfillPreview() {
  backfillPreviewLoading.value = true;
  try {
    backfillPreview.value = await api.getBackfillPreview();
  } catch {
    // toast already shown
  } finally {
    backfillPreviewLoading.value = false;
  }
}

async function runLegacyEmbeddingsBackfill() {
  runningLegacyEmbeddingsBackfill.value = true;
  try {
    await api.runLegacyEmbeddingsBackfill();
    toastsStore.add("Legacy embeddings backfill started", "success");
  } catch {
    // toast already shown
  } finally {
    runningLegacyEmbeddingsBackfill.value = false;
  }
}

async function runRecordBackfill() {
  runningRecordBackfill.value = true;
  try {
    await api.runRecordBackfill();
    toastsStore.add("Record backfill started", "success");
    await loadBackfillPreview();
  } catch {
    // toast already shown
  } finally {
    runningRecordBackfill.value = false;
  }
}

async function runArtifactBackfill() {
  runningArtifactBackfill.value = true;
  try {
    await api.runArtifactBackfill();
    toastsStore.add("Artifact backfill started", "success");
    await loadBackfillPreview();
  } catch {
    // toast already shown
  } finally {
    runningArtifactBackfill.value = false;
  }
}

async function runSuggestionsBackfill() {
  runningSuggestionsBackfill.value = true;
  try {
    await api.runSuggestionsBackfill();
    toastsStore.add("Suggestions backfill started", "success");
    await loadBackfillPreview();
  } catch {
    // toast already shown
  } finally {
    runningSuggestionsBackfill.value = false;
  }
}

async function loadSuggestionJobs() {
  suggestionJobsLoading.value = true;
  try {
    const result = await api.getSuggestionJobs({
      all: suggestionJobsShowAll.value,
      status: suggestionJobsStatusFilter.value || undefined,
      limit: suggestionJobsPageSize,
      offset: suggestionJobsPage.value * suggestionJobsPageSize,
    });
    suggestionJobsTotal.value = result.total;
    if (result.jobs.length === 0 && result.total > 0) {
      suggestionJobsPage.value = Math.max(
        0,
        Math.ceil(result.total / suggestionJobsPageSize) - 1,
      );
      return loadSuggestionJobs();
    }
    suggestionJobs.value = result.jobs;
  } catch {
    // toast already shown
  } finally {
    suggestionJobsLoading.value = false;
  }
}

function suggestionJobsSetPage(page: number) {
  suggestionJobsPage.value = page;
  suggestionJobsPageInput.value = page + 1;
  loadSuggestionJobs();
}

async function clearSuggestionJobsByStatus(status: string) {
  try {
    await api.deleteBulkSuggestionJobs(status, suggestionJobsShowAll.value);
    await loadSuggestionJobs();
  } catch {
    // toast already shown
  }
}

async function deleteSuggestionJob(id: number) {
  try {
    await api.deleteSuggestionJob(id);
    suggestionJobs.value = suggestionJobs.value.filter((j) => j.id !== id);
  } catch {
    // toast already shown
  }
}

async function invalidateEmbeddings() {
  if (
    !confirm(
      "Delete all your embeddings? They will be regenerated on next search or access.",
    )
  )
    return;
  invalidatingEmbeddings.value = true;
  try {
    await api.invalidateUserEmbeddings();
    toastsStore.add("Embeddings invalidated", "success");
  } catch {
    // toast already shown
  } finally {
    invalidatingEmbeddings.value = false;
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

async function loadJobs() {
  jobsLoading.value = true;
  try {
    const result = await api.getEmbeddingJobs({
      all: jobsShowAll.value,
      status: jobsStatusFilter.value || undefined,
      limit: jobsPageSize,
      offset: jobsPage.value * jobsPageSize,
    });
    jobsTotal.value = result.total;
    // If the current page is past the last page with data, jump to the last valid page.
    if (result.jobs.length === 0 && result.total > 0) {
      jobsPage.value = Math.max(0, Math.ceil(result.total / jobsPageSize) - 1);
      return loadJobs();
    }
    jobs.value = result.jobs;
  } catch {
    // toast already shown
  } finally {
    jobsLoading.value = false;
  }
}

function jobsSetPage(page: number) {
  jobsPage.value = page;
  jobsPageInput.value = page + 1;
  loadJobs();
}

async function clearJobsByStatus(status: string) {
  try {
    await api.deleteBulkEmbeddingJobs(status, jobsShowAll.value);
    await loadJobs();
  } catch {
    // toast already shown
  }
}

async function deleteJob(id: number) {
  try {
    await api.deleteEmbeddingJob(id);
    jobs.value = jobs.value.filter((j) => j.id !== id);
  } catch {
    // toast already shown
  }
}

let jobsReloadTimer: ReturnType<typeof setTimeout> | null = null;
watch(
  () => recordsStore.embeddingProgressTick,
  () => {
    if (activeTab.value !== "jobs") return;
    if (jobsReloadTimer) clearTimeout(jobsReloadTimer);
    jobsReloadTimer = setTimeout(() => loadJobs(), 500);
  },
);

async function toggleAdmin(user: {
  id: number;
  username: string;
  isAdmin: boolean;
}) {
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
  router.replace({ name: "settings", params: { tab } });
  if (tab === "user") loadUserConfig();
  else if (tab === "global") {
    loadGlobalConfig();
    loadBackfillPreview();
    loadOllamaModels();
  } else if (tab === "users") loadUsers();
  else if (tab === "jobs") loadJobs();
  else if (tab === "suggestion-jobs") loadSuggestionJobs();
}

onMounted(() => {
  loadCapabilities();
  const tab = tabFromRoute();
  activeTab.value = tab;
  if (tab === "user") loadUserConfig();
  else if (tab === "global") {
    loadGlobalConfig();
    loadBackfillPreview();
    loadOllamaModels();
  } else if (tab === "users") loadUsers();
  else if (tab === "jobs") loadJobs();
  else if (tab === "suggestion-jobs") loadSuggestionJobs();
});
</script>

<template>
  <div
    class="min-h-screen bg-gray-50 text-gray-900 dark:bg-gray-900 dark:text-white"
  >
    <!-- Header -->
    <div class="container mx-auto px-4 pt-4">
      <div class="mb-6 flex items-center gap-4">
        <button
          @click="router.push({ path: '/' })"
          class="text-sm text-blue-600 hover:underline dark:text-sky-400"
        >
          ← Back
        </button>
        <h1 class="text-2xl font-semibold">Settings</h1>
        <span
          v-if="currentUsername"
          class="ml-auto text-sm text-gray-500 dark:text-gray-400"
        >
          Signed in as <strong>{{ currentUsername }}</strong>
          <span
            v-if="isAdmin"
            class="ml-2 rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900 dark:text-blue-300"
            >admin</span
          >
        </span>
      </div>

      <!-- Tabs -->
      <div
        class="mb-6 flex gap-1 border-b border-gray-200 dark:border-gray-700"
      >
        <button
          v-for="tab in isAdmin
            ? (['user', 'global', 'users', 'jobs', 'suggestion-jobs'] as Tab[])
            : (['user', 'jobs', 'suggestion-jobs'] as Tab[])"
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
                : tab === "users"
                  ? "Users"
                  : tab === "jobs"
                    ? "Embedding Jobs"
                    : "Suggestion Jobs"
          }}
        </button>
      </div>

      <!-- User Settings Tab -->
      <div v-if="activeTab === 'user'" class="max-w-lg">
        <div v-if="userConfigLoading" class="text-gray-500">Loading…</div>
        <form
          v-else
          @submit.prevent="saveUserConfig"
          class="flex flex-col gap-4"
        >
          <p class="text-sm text-gray-500 dark:text-gray-400">
            These settings control how your records and images are indexed for
            search. Leave a field blank to inherit the server default shown in
            the placeholder.
          </p>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Text embedding model</label
            >
            <input
              v-model="userConfig.infinityTextModel"
              type="text"
              :placeholder="globalConfig.infinityTextModel || 'Server default'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Model used to embed record titles and descriptions for text
              search.
            </p>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Image embedding model</label
            >
            <input
              v-model="userConfig.infinityImageModel"
              type="text"
              :placeholder="globalConfig.infinityImageModel || 'Server default'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Model used to embed artifact images for image similarity search.
            </p>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
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
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Prepended to search queries before embedding. Some models require
              a task-specific prefix to return good results (e.g.
              <span class="font-mono">Represent this sentence…</span>).
            </p>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Text document prefix</label
            >
            <input
              v-model="userConfig.infinityTextDocumentPrefix"
              type="text"
              :placeholder="globalConfig.infinityTextDocumentPrefix || 'None'"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Prepended to record text before embedding at index time. Usually
              blank unless the model requires asymmetric input.
            </p>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Maximum embedding dimensions</label
            >
            <input
              :value="userConfig.maximumEmbeddingDimensions ?? ''"
              @input="
                (e) => {
                  const v = (e.target as HTMLInputElement).value;
                  userConfig.maximumEmbeddingDimensions =
                    v === '' ? null : Math.max(1, parseInt(v) || 1);
                }
              "
              type="number"
              min="1"
              :placeholder="
                globalConfig.maximumEmbeddingDimensions
                  ? String(globalConfig.maximumEmbeddingDimensions)
                  : 'Model default'
              "
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Caps embedding dimensions sent to Infinity for your account. Leave
              blank to inherit the global setting.
            </p>
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Barcode / QR code detection
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            When images are uploaded, they are scanned for barcodes and QR
            codes. By default your account uses the formats configured globally.
            Enable the override below to use a different set for your account
            only.
          </p>

          <div class="flex items-center gap-3">
            <input
              id="barcodeOverride"
              type="checkbox"
              :checked="userConfig.enabledBarcodeFormats !== null"
              @change="
                userConfig.enabledBarcodeFormats =
                  userConfig.enabledBarcodeFormats !== null
                    ? null
                    : [...(globalConfig.enabledBarcodeFormats ?? [])]
              "
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="barcodeOverride" class="text-sm font-medium"
              >Use a custom format list for my account</label
            >
          </div>

          <template v-if="userConfig.enabledBarcodeFormats !== null">
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
              <div
                v-for="fmt in allBarcodeFormats"
                :key="fmt.value"
                class="flex items-center gap-2"
              >
                <input
                  :id="`user-barcode-${fmt.value}`"
                  type="checkbox"
                  :checked="
                    userConfig.enabledBarcodeFormats!.includes(fmt.value)
                  "
                  @change="
                    userConfig.enabledBarcodeFormats!.includes(fmt.value)
                      ? userConfig.enabledBarcodeFormats!.splice(
                          userConfig.enabledBarcodeFormats!.indexOf(fmt.value),
                          1,
                        )
                      : userConfig.enabledBarcodeFormats!.push(fmt.value)
                  "
                  class="h-4 w-4 rounded border-gray-300 text-blue-600"
                />
                <label
                  :for="`user-barcode-${fmt.value}`"
                  class="text-sm font-medium"
                  >{{ fmt.label }}</label
                >
              </div>
            </div>
            <p
              v-if="userConfig.enabledBarcodeFormats.length === 0"
              class="text-sm text-amber-600 dark:text-amber-400"
            >
              No formats selected — barcode scanning is disabled for your
              account.
            </p>
            <p v-else class="text-xs text-gray-500 dark:text-gray-400">
              Overrides the global setting for your account only.
            </p>
          </template>
          <template v-else>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              <template
                v-if="
                  globalConfig.enabledBarcodeFormats &&
                  globalConfig.enabledBarcodeFormats.length > 0
                "
              >
                Using global defaults:
                {{
                  globalConfig.enabledBarcodeFormats
                    .map(
                      (v) =>
                        allBarcodeFormats.find((f) => f.value === v)?.label ??
                        v,
                    )
                    .join(", ")
                }}.
              </template>
              <template v-else>
                Barcode scanning is disabled globally. Enable formats in Global
                Settings or override them here.
              </template>
            </p>
          </template>

          <div class="flex gap-3">
            <button
              type="submit"
              :disabled="userConfigSaving"
              class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {{ userConfigSaving ? "Saving…" : "Save" }}
            </button>
            <button
              type="button"
              :disabled="invalidatingEmbeddings"
              @click="invalidateEmbeddings"
              class="rounded-lg bg-red-100 px-4 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-200 disabled:opacity-50 dark:bg-red-900/40 dark:text-red-400 dark:hover:bg-red-900"
            >
              {{
                invalidatingEmbeddings
                  ? "Invalidating…"
                  : "Invalidate my embeddings"
              }}
            </button>
          </div>
        </form>
      </div>

      <!-- Global Settings Tab -->
      <div v-if="activeTab === 'global'" class="max-w-lg">
        <div v-if="globalConfigLoading" class="text-gray-500">Loading…</div>
        <form
          v-else
          @submit.prevent="saveGlobalConfig"
          class="flex flex-col gap-4"
        >
          <div>
            <label class="mb-1 block text-sm font-medium">Log level</label>
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
              v-model="globalConfig.backfillLegacyEmbeddingsOnStart"
              id="backfillLegacyEmbeddings"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            <label for="backfillLegacyEmbeddings" class="text-sm font-medium"
              >Re-index legacy embeddings on start</label
            >
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
              v-model="globalConfig.backfillSuggestionsOnStart"
              id="backfillSuggestions"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-purple-600"
            />
            <label for="backfillSuggestions" class="text-sm font-medium"
              >Backfill Ollama content suggestions on start</label
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
            <label class="mb-1 block text-sm font-medium"
              >Text embedding model</label
            >
            <input
              v-model="globalConfig.infinityTextModel"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Image embedding model</label
            >
            <input
              v-model="globalConfig.infinityImageModel"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Text query prefix</label
            >
            <input
              v-model="globalConfig.infinityTextQueryPrefix"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Text document prefix</label
            >
            <input
              v-model="globalConfig.infinityTextDocumentPrefix"
              type="text"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Maximum embedding dimensions</label
            >
            <input
              :value="globalConfig.maximumEmbeddingDimensions ?? ''"
              @input="
                (e) => {
                  const v = (e.target as HTMLInputElement).value;
                  globalConfig.maximumEmbeddingDimensions =
                    v === '' ? null : Math.max(1, parseInt(v) || 1);
                }
              "
              type="number"
              min="1"
              placeholder="Model default"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Caps embedding dimensions sent to Infinity server-wide. Leave
              blank to use whatever the model provides.
            </p>
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Ollama content suggestions
          </p>

          <div>
            <label class="mb-1 block text-sm font-medium">Ollama address</label>
            <input
              v-model="globalConfig.ollamaAddress"
              type="text"
              placeholder="http://localhost:11434"
              class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
              @blur="loadOllamaModels"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium"
              >Ollama vision model</label
            >
            <div class="flex gap-2">
              <select
                v-if="ollamaModels.length > 0"
                v-model="globalConfig.ollamaVisionModel"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
              >
                <option value="" disabled>Select a model…</option>
                <option
                  v-for="model in ollamaModels"
                  :key="model"
                  :value="model"
                >
                  {{ model }}
                </option>
              </select>
              <input
                v-else
                v-model="globalConfig.ollamaVisionModel"
                type="text"
                placeholder="qwen3.5:2b"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
              />
              <button
                type="button"
                @click="loadOllamaModels"
                title="Refresh model list"
                class="shrink-0 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:hover:bg-gray-700"
              >
                ↺
              </button>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Vision model used for the "Suggest" button when creating records.
              <span
                v-if="ollamaModels.length > 0"
                class="text-green-600 dark:text-green-400"
              >
                {{ ollamaModels.length }} model{{
                  ollamaModels.length !== 1 ? "s" : ""
                }}
                available.
              </span>
            </p>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Pull model</label>
            <div class="flex gap-2">
              <input
                v-model="ollamaPullModel"
                type="text"
                placeholder="e.g. qwen3.5:2b, llama3.2-vision"
                :disabled="ollamaPulling"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm disabled:opacity-50 dark:border-gray-600 dark:bg-gray-800"
                @keydown.enter.prevent="pullOllamaModel"
              />
              <button
                type="button"
                @click="pullOllamaModel"
                :disabled="ollamaPulling || !ollamaPullModel.trim()"
                class="shrink-0 rounded-lg bg-purple-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-purple-700 disabled:opacity-50"
              >
                {{ ollamaPulling ? "Pulling…" : "Pull" }}
              </button>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Downloads the model from the Ollama registry. You will be notified
              when complete.
            </p>
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">
            Barcode / QR code detection
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            Formats to scan for when images are uploaded. Uncheck all to
            disable.
          </p>

          <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <div
              v-for="fmt in allBarcodeFormats"
              :key="fmt.value"
              class="flex items-center gap-2"
            >
              <input
                :id="`barcode-${fmt.value}`"
                type="checkbox"
                :checked="isBarcodeFormatEnabled(fmt.value)"
                @change="toggleBarcodeFormat(fmt.value)"
                class="h-4 w-4 rounded border-gray-300 text-blue-600"
              />
              <label
                :for="`barcode-${fmt.value}`"
                class="text-sm font-medium"
                >{{ fmt.label }}</label
              >
            </div>
          </div>

          <div>
            <button
              type="submit"
              :disabled="globalConfigSaving"
              class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {{ globalConfigSaving ? "Saving…" : "Save" }}
            </button>
          </div>

          <hr class="border-gray-200 dark:border-gray-700" />
          <p class="text-xs text-gray-500 dark:text-gray-400">Backfills</p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            Enqueue embedding jobs for items that have not yet been indexed.
          </p>

          <div
            class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50"
          >
            <div>
              <p class="text-sm font-medium">Legacy embeddings</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                <template v-if="backfillPreviewLoading">Counting…</template>
                <template v-else-if="backfillPreview">
                  {{ backfillPreview.legacyEmbeddings }} legacy embedding{{
                    backfillPreview.legacyEmbeddings !== 1 ? "s" : ""
                  }}
                  to delete
                </template>
                <template v-else
                  >Delete old JSON-format embeddings so they are
                  regenerated</template
                >
              </p>
            </div>
            <button
              type="button"
              @click="runLegacyEmbeddingsBackfill"
              :disabled="runningLegacyEmbeddingsBackfill"
              class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {{ runningLegacyEmbeddingsBackfill ? "Starting…" : "Run" }}
            </button>
          </div>

          <div
            class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50"
          >
            <div>
              <p class="text-sm font-medium">Record text embeddings</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                <template v-if="backfillPreviewLoading">Counting…</template>
                <template v-else-if="backfillPreview">
                  {{ backfillPreview.records }} record{{
                    backfillPreview.records !== 1 ? "s" : ""
                  }}
                  without embeddings
                </template>
              </p>
            </div>
            <button
              type="button"
              @click="runRecordBackfill"
              :disabled="runningRecordBackfill || backfillPreviewLoading"
              class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {{ runningRecordBackfill ? "Starting…" : "Run" }}
            </button>
          </div>

          <div
            class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50"
          >
            <div>
              <p class="text-sm font-medium">Artifact image embeddings</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                <template v-if="backfillPreviewLoading">Counting…</template>
                <template v-else-if="backfillPreview">
                  {{ backfillPreview.artifacts }} artifact{{
                    backfillPreview.artifacts !== 1 ? "s" : ""
                  }}
                  without embeddings
                </template>
              </p>
            </div>
            <button
              type="button"
              @click="runArtifactBackfill"
              :disabled="runningArtifactBackfill || backfillPreviewLoading"
              class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50"
            >
              {{ runningArtifactBackfill ? "Starting…" : "Run" }}
            </button>
          </div>

          <div
            class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-700 dark:bg-gray-800/50"
          >
            <div>
              <p class="text-sm font-medium">Ollama content suggestions</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                <template v-if="backfillPreviewLoading">Counting…</template>
                <template v-else-if="backfillPreview">
                  {{ backfillPreview.suggestions }} artifact{{
                    backfillPreview.suggestions !== 1 ? "s" : ""
                  }}
                  without suggestions
                </template>
              </p>
            </div>
            <button
              type="button"
              @click="runSuggestionsBackfill"
              :disabled="runningSuggestionsBackfill || backfillPreviewLoading"
              class="rounded-lg bg-purple-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-purple-700 disabled:opacity-50"
            >
              {{ runningSuggestionsBackfill ? "Starting…" : "Run" }}
            </button>
          </div>
        </form>
      </div>

      <!-- Embedding Jobs Tab -->
      <div v-if="activeTab === 'jobs'" class="max-w-3xl">
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <select
            v-model="jobsStatusFilter"
            @change="
              jobsPage = 0;
              loadJobs();
            "
            class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
          >
            <option value="">All statuses</option>
            <option value="pending">Pending</option>
            <option value="processing">Processing</option>
            <option value="done">Done</option>
            <option value="failed">Failed</option>
          </select>
          <label v-if="isAdmin" class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              v-model="jobsShowAll"
              @change="
                jobsPage = 0;
                loadJobs();
              "
              class="h-4 w-4 rounded border-gray-300 text-blue-600"
            />
            Show all users
          </label>
          <div class="ml-auto flex gap-2">
            <button
              @click="clearJobsByStatus('pending')"
              :disabled="jobsLoading"
              class="rounded-lg bg-red-100 px-3 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-200 disabled:opacity-50 dark:bg-red-900/40 dark:text-red-400 dark:hover:bg-red-900"
            >
              Clear pending
            </button>
            <button
              @click="clearJobsByStatus('done')"
              :disabled="jobsLoading"
              class="rounded-lg bg-green-100 px-3 py-2 text-sm font-medium text-green-700 transition-colors hover:bg-green-200 disabled:opacity-50 dark:bg-green-900/40 dark:text-green-400 dark:hover:bg-green-900"
            >
              Clear done
            </button>
            <button
              @click="clearJobsByStatus('failed')"
              :disabled="jobsLoading"
              class="rounded-lg bg-orange-100 px-3 py-2 text-sm font-medium text-orange-700 transition-colors hover:bg-orange-200 disabled:opacity-50 dark:bg-orange-900/40 dark:text-orange-400 dark:hover:bg-orange-900"
            >
              Clear failed
            </button>
            <button
              @click="loadJobs"
              :disabled="jobsLoading"
              class="rounded-lg bg-gray-100 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:opacity-50 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
            >
              {{ jobsLoading ? "Loading…" : "Refresh" }}
            </button>
          </div>
        </div>
        <div v-if="jobsLoading && jobs.length === 0" class="text-gray-500">
          Loading…
        </div>
        <div v-else-if="jobs.length === 0" class="text-sm text-gray-500">
          No embedding jobs found.
        </div>
        <div
          v-else
          class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700"
        >
          <table class="w-full text-xs">
            <thead>
              <tr
                class="border-b border-gray-200 bg-white text-left text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
              >
                <th class="px-3 py-2 font-semibold">Type</th>
                <th class="px-3 py-2 font-semibold">ID</th>
                <th class="px-3 py-2 font-semibold">Model</th>
                <th class="px-3 py-2 font-semibold">Dims</th>
                <th class="px-3 py-2 font-semibold">User</th>
                <th class="px-3 py-2 font-semibold">Source</th>
                <th class="px-3 py-2 font-semibold">Status</th>
                <th class="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody
              class="divide-y divide-gray-100 bg-white dark:divide-gray-700/50 dark:bg-gray-800"
            >
              <template v-for="job in jobs" :key="job.id">
                <tr>
                  <td
                    class="px-3 py-2 font-mono text-gray-500 dark:text-gray-400"
                  >
                    {{ job.jobType }}
                  </td>
                  <td
                    class="px-3 py-2 font-mono text-gray-500 dark:text-gray-400"
                  >
                    {{ job.targetID }}
                  </td>
                  <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                    {{ job.embedModel }}
                  </td>
                  <td
                    class="px-3 py-2 font-mono text-gray-500 dark:text-gray-400"
                  >
                    {{ job.dimensions ?? "—" }}
                  </td>
                  <td class="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {{ job.username || "—" }}
                  </td>
                  <td class="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {{ job.source }}
                  </td>
                  <td class="px-3 py-2">
                    <span
                      :class="[
                        'rounded-full px-2 py-0.5 font-medium',
                        job.status === 'done'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400'
                          : job.status === 'failed'
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400'
                            : job.status === 'processing'
                              ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400'
                              : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400',
                      ]"
                      >{{ job.status }}</span
                    >
                  </td>
                  <td class="px-3 py-2">
                    <button
                      v-if="job.status === 'pending'"
                      @click="deleteJob(job.id)"
                      class="rounded px-1.5 py-0.5 text-gray-400 transition-colors hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900/40 dark:hover:text-red-400"
                      title="Delete job"
                    >
                      ✕
                    </button>
                  </td>
                </tr>
                <tr v-if="job.errorMsg">
                  <td
                    colspan="8"
                    class="px-3 pb-2 text-red-600 dark:text-red-400"
                  >
                    {{ job.errorMsg }}
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div
          v-if="jobsTotal > jobsPageSize"
          class="mt-3 flex items-center justify-between text-sm text-gray-600 dark:text-gray-400"
        >
          <span
            >{{ jobsPage * jobsPageSize + 1 }}–{{
              Math.min((jobsPage + 1) * jobsPageSize, jobsTotal)
            }}
            of {{ jobsTotal }}</span
          >
          <div class="flex items-center gap-1">
            <button
              @click="jobsSetPage(0)"
              :disabled="jobsPage === 0"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              title="First page"
            >
              «
            </button>
            <button
              @click="jobsSetPage(jobsPage - 1)"
              :disabled="jobsPage === 0"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
            >
              ←
            </button>
            <input
              type="number"
              :min="1"
              :max="Math.ceil(jobsTotal / jobsPageSize)"
              v-model.number="jobsPageInput"
              @change="
                jobsSetPage(
                  Math.min(
                    Math.max(0, (isNaN(jobsPageInput) ? 1 : jobsPageInput) - 1),
                    Math.ceil(jobsTotal / jobsPageSize) - 1,
                  ),
                )
              "
              class="w-14 rounded-lg border border-gray-300 bg-white px-2 py-1 text-center text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <span class="text-gray-400"
              >/ {{ Math.ceil(jobsTotal / jobsPageSize) }}</span
            >
            <button
              @click="jobsSetPage(jobsPage + 1)"
              :disabled="(jobsPage + 1) * jobsPageSize >= jobsTotal"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
            >
              →
            </button>
            <button
              @click="jobsSetPage(Math.ceil(jobsTotal / jobsPageSize) - 1)"
              :disabled="(jobsPage + 1) * jobsPageSize >= jobsTotal"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              title="Last page"
            >
              »
            </button>
          </div>
        </div>
      </div>

      <!-- Suggestion Jobs Tab -->
      <div v-if="activeTab === 'suggestion-jobs'" class="max-w-3xl">
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <select
            v-model="suggestionJobsStatusFilter"
            @change="
              suggestionJobsPage = 0;
              loadSuggestionJobs();
            "
            class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-800"
          >
            <option value="">All statuses</option>
            <option value="pending">Pending</option>
            <option value="processing">Processing</option>
            <option value="done">Done</option>
            <option value="failed">Failed</option>
          </select>
          <label v-if="isAdmin" class="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              v-model="suggestionJobsShowAll"
              @change="
                suggestionJobsPage = 0;
                loadSuggestionJobs();
              "
              class="h-4 w-4 rounded border-gray-300 text-purple-600"
            />
            Show all users
          </label>
          <div class="ml-auto flex gap-2">
            <button
              @click="clearSuggestionJobsByStatus('pending')"
              :disabled="suggestionJobsLoading"
              class="rounded-lg bg-red-100 px-3 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-200 disabled:opacity-50 dark:bg-red-900/40 dark:text-red-400 dark:hover:bg-red-900"
            >
              Clear pending
            </button>
            <button
              @click="clearSuggestionJobsByStatus('done')"
              :disabled="suggestionJobsLoading"
              class="rounded-lg bg-green-100 px-3 py-2 text-sm font-medium text-green-700 transition-colors hover:bg-green-200 disabled:opacity-50 dark:bg-green-900/40 dark:text-green-400 dark:hover:bg-green-900"
            >
              Clear done
            </button>
            <button
              @click="clearSuggestionJobsByStatus('failed')"
              :disabled="suggestionJobsLoading"
              class="rounded-lg bg-orange-100 px-3 py-2 text-sm font-medium text-orange-700 transition-colors hover:bg-orange-200 disabled:opacity-50 dark:bg-orange-900/40 dark:text-orange-400 dark:hover:bg-orange-900"
            >
              Clear failed
            </button>
            <button
              @click="loadSuggestionJobs"
              :disabled="suggestionJobsLoading"
              class="rounded-lg bg-gray-100 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 disabled:opacity-50 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
            >
              {{ suggestionJobsLoading ? "Loading…" : "Refresh" }}
            </button>
          </div>
        </div>
        <div
          v-if="suggestionJobsLoading && suggestionJobs.length === 0"
          class="text-gray-500"
        >
          Loading…
        </div>
        <div
          v-else-if="suggestionJobs.length === 0"
          class="text-sm text-gray-500"
        >
          No suggestion jobs found.
        </div>
        <div
          v-else
          class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700"
        >
          <table class="w-full text-xs">
            <thead>
              <tr
                class="border-b border-gray-200 bg-white text-left text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
              >
                <th class="px-3 py-2 font-semibold">Artifact ID</th>
                <th class="px-3 py-2 font-semibold">Model</th>
                <th class="px-3 py-2 font-semibold">User</th>
                <th class="px-3 py-2 font-semibold">Source</th>
                <th class="px-3 py-2 font-semibold">Status</th>
                <th class="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody
              class="divide-y divide-gray-100 bg-white dark:divide-gray-700/50 dark:bg-gray-800"
            >
              <template v-for="job in suggestionJobs" :key="job.id">
                <tr>
                  <td
                    class="px-3 py-2 font-mono text-gray-500 dark:text-gray-400"
                  >
                    {{ job.artifactID }}
                  </td>
                  <td class="px-3 py-2 text-gray-700 dark:text-gray-300">
                    {{ job.ollamaModel }}
                  </td>
                  <td class="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {{ job.username || "—" }}
                  </td>
                  <td class="px-3 py-2 text-gray-500 dark:text-gray-400">
                    {{ job.source }}
                  </td>
                  <td class="px-3 py-2">
                    <span
                      :class="[
                        'rounded-full px-2 py-0.5 font-medium',
                        job.status === 'done'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400'
                          : job.status === 'failed'
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400'
                            : job.status === 'processing'
                              ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-400'
                              : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400',
                      ]"
                      >{{ job.status }}</span
                    >
                  </td>
                  <td class="px-3 py-2">
                    <button
                      v-if="job.status === 'pending'"
                      @click="deleteSuggestionJob(job.id)"
                      class="rounded px-1.5 py-0.5 text-gray-400 transition-colors hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900/40 dark:hover:text-red-400"
                      title="Delete job"
                    >
                      ✕
                    </button>
                  </td>
                </tr>
                <tr v-if="job.errorMsg">
                  <td
                    colspan="6"
                    class="px-3 pb-2 text-red-600 dark:text-red-400"
                  >
                    {{ job.errorMsg }}
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div
          v-if="suggestionJobsTotal > suggestionJobsPageSize"
          class="mt-3 flex items-center justify-between text-sm text-gray-600 dark:text-gray-400"
        >
          <span
            >{{ suggestionJobsPage * suggestionJobsPageSize + 1 }}–{{
              Math.min(
                (suggestionJobsPage + 1) * suggestionJobsPageSize,
                suggestionJobsTotal,
              )
            }}
            of {{ suggestionJobsTotal }}</span
          >
          <div class="flex items-center gap-1">
            <button
              @click="suggestionJobsSetPage(0)"
              :disabled="suggestionJobsPage === 0"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              title="First page"
            >
              «
            </button>
            <button
              @click="suggestionJobsSetPage(suggestionJobsPage - 1)"
              :disabled="suggestionJobsPage === 0"
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
            >
              ←
            </button>
            <input
              type="number"
              :min="1"
              :max="Math.ceil(suggestionJobsTotal / suggestionJobsPageSize)"
              v-model.number="suggestionJobsPageInput"
              @change="
                suggestionJobsSetPage(
                  Math.min(
                    Math.max(
                      0,
                      (isNaN(suggestionJobsPageInput)
                        ? 1
                        : suggestionJobsPageInput) - 1,
                    ),
                    Math.ceil(suggestionJobsTotal / suggestionJobsPageSize) - 1,
                  ),
                )
              "
              class="w-14 rounded-lg border border-gray-300 bg-white px-2 py-1 text-center text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <span class="text-gray-400"
              >/
              {{
                Math.ceil(suggestionJobsTotal / suggestionJobsPageSize)
              }}</span
            >
            <button
              @click="suggestionJobsSetPage(suggestionJobsPage + 1)"
              :disabled="
                (suggestionJobsPage + 1) * suggestionJobsPageSize >=
                suggestionJobsTotal
              "
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
            >
              →
            </button>
            <button
              @click="
                suggestionJobsSetPage(
                  Math.ceil(suggestionJobsTotal / suggestionJobsPageSize) - 1,
                )
              "
              :disabled="
                (suggestionJobsPage + 1) * suggestionJobsPageSize >=
                suggestionJobsTotal
              "
              class="rounded-lg px-3 py-1 text-sm transition-colors hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-gray-700"
              title="Last page"
            >
              »
            </button>
          </div>
        </div>
      </div>

      <!-- Users Tab -->
      <div v-if="activeTab === 'users'" class="max-w-lg">
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
                <button
                  @click="toggleAdmin(user)"
                  :disabled="user.isAdmin && user.username === currentUsername"
                  :title="
                    user.isAdmin && user.username === currentUsername
                      ? 'Cannot remove admin from yourself'
                      : undefined
                  "
                  :class="[
                    'rounded-lg px-3 py-1 text-xs font-medium transition-colors',
                    user.isAdmin && user.username === currentUsername
                      ? 'cursor-not-allowed bg-red-100 text-red-700 opacity-40 dark:bg-red-900/40 dark:text-red-400'
                      : user.isAdmin
                        ? 'bg-red-100 text-red-700 hover:bg-red-200 dark:bg-red-900/40 dark:text-red-400 dark:hover:bg-red-900'
                        : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600',
                  ]"
                >
                  {{ user.isAdmin ? "Remove admin" : "Make admin" }}
                </button>
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
