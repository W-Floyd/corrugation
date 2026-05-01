<script setup lang="ts" name="SearchBar">
import { ref, onBeforeUnmount, watch } from "vue";
import KbdHint from "@/components/KbdHint.vue";
import MagnifyIcon from "vue-material-design-icons/Magnify.vue";
import CloseIcon from "vue-material-design-icons/Close.vue";
import EarthIcon from "vue-material-design-icons/Earth.vue";
import TextBoxSearchIcon from "vue-material-design-icons/TextBoxSearch.vue";
import ImageSearchIcon from "vue-material-design-icons/ImageSearch.vue";
import TextSearchIcon from "vue-material-design-icons/TextSearch.vue";
import { useRecordsStore } from "@/stores/records";

const props = defineProps<{ showHint?: boolean }>();

const recordsStore = useRecordsStore();

const debounceTimer = ref<ReturnType<typeof setTimeout> | null>(null);
const searchInputEl = ref<HTMLInputElement | null>(null);

const focusSearch = (): void => {
  searchInputEl.value?.focus();
  searchInputEl.value?.select();
};

const handleSearchInput = (): void => {
  if (debounceTimer.value) {
    clearTimeout(debounceTimer.value);
  }

  debounceTimer.value = setTimeout(() => {
    recordsStore.debouncesearch();
    recordsStore.searching = true;
  }, 500);
};

const onWorldChange = (): void => {
  if (recordsStore.searchtext.trim() !== "") {
    recordsStore.searching = true;
  }
};

const resetSearch = (): void => {
  recordsStore.searchtext = "";
  recordsStore.searchtextpredebounce = "";
  recordsStore.searching = false;
  searchInputEl.value?.blur();
};

watch(
  () => recordsStore.searchtext,
  (newVal, oldVal) => {
    if (oldVal !== undefined && oldVal !== "" && newVal === "") {
      recordsStore.selectedRecordId = null;
    }
  },
);

defineExpose({ focusSearch });

onBeforeUnmount(() => {
  if (debounceTimer.value) {
    clearTimeout(debounceTimer.value);
  }
});
</script>

<template>
  <div class="mb-4 flex flex-row flex-wrap items-center gap-2">
    <!-- Search icon -->
    <div class="text-gray-500 dark:text-gray-400">
      <MagnifyIcon :size="24" />
    </div>

    <!-- Filter world checkbox -->
    <div class="flex items-center">
      <label
        class="relative flex cursor-pointer items-center"
        title="Only search in current record"
      >
        <input
          type="checkbox"
          v-model="recordsStore.filterworld"
          class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
          @change="onWorldChange"
        />
        <EarthIcon
          class="relative ml-1 text-sm text-gray-600 dark:text-gray-400"
          :size="16"
        />
        <KbdHint contents="G" :show="props.showHint" :center="true" />
      </label>
    </div>

    <!-- Text embedding toggle -->
    <div class="flex shrink-0 items-center">
      <label
        class="relative flex cursor-pointer items-center"
        title="Use text embeddings in search"
      >
        <input
          type="checkbox"
          v-model="recordsStore.searchTextEmbedded"
          class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        <TextBoxSearchIcon
          class="relative ml-1 text-sm text-gray-600 dark:text-gray-400"
          :size="16"
        />
        <KbdHint contents="W" :show="props.showHint" :center="true" />
      </label>
    </div>

    <!-- String matching toggle -->
    <div class="flex items-center">
      <label
        class="relative flex cursor-pointer items-center"
        title="Use substring matching in search"
      >
        <input
          type="checkbox"
          v-model="recordsStore.searchTextSubstring"
          class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        <TextSearchIcon
          class="relative ml-1 text-sm text-gray-600 dark:text-gray-400"
          :size="16"
        />
        <KbdHint contents="T" :show="props.showHint" :center="true" />
      </label>
    </div>

    <!-- Image embedding toggle -->
    <div class="flex items-center">
      <label
        class="relative flex cursor-pointer items-center"
        title="Use image embeddings in search"
      >
        <input
          type="checkbox"
          v-model="recordsStore.searchImage"
          class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
        />
        <ImageSearchIcon
          class="relative ml-1 text-sm text-gray-600 dark:text-gray-400"
          :size="16"
        />
        <KbdHint contents="I" :show="props.showHint" :center="true" />
      </label>
    </div>

    <!-- Search input -->
    <div class="relative min-w-xs flex-1">
      <input
        ref="searchInputEl"
        v-model="recordsStore.searchtextpredebounce"
        @input="handleSearchInput"
        @keydown.enter.stop="searchInputEl?.blur()"
        @keydown.esc.stop="searchInputEl?.blur()"
        placeholder="Search for a record..."
        type="search"
        class="w-full rounded-full bg-white px-4 py-2 pr-14 ring-1 ring-gray-300 focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-800 dark:text-white dark:ring-gray-600"
      />
      <kbd
        v-if="props.showHint"
        class="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 rounded bg-gray-800 px-1 font-sans text-[9px] leading-3.5 text-white shadow"
        >/
      </kbd>
      <button
        v-if="recordsStore.searchtext"
        @click="resetSearch"
        type="button"
        class="absolute top-1/2 right-2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
        title="Clear search"
      >
        <CloseIcon :size="14" />
      </button>
      <!-- Image search indicator badge -->
      <div
        v-if="
          recordsStore.apiSearchResults.length > 0 &&
          recordsStore.searchtext === '🔍'
        "
        class="absolute top-1/2 right-10 flex -translate-y-1/2 items-center gap-1"
      >
        <div
          class="flex items-center gap-1 rounded-full bg-blue-50 px-2 py-1 text-xs text-blue-600 dark:bg-blue-900/20 dark:text-blue-400"
        >
          <ImageSearchIcon :size="12" />
          <span class="font-semibold">{{
            recordsStore.apiSearchResults.length
          }}</span>
          <span class="text-[10px] text-gray-500 dark:text-gray-400"
            >similar</span
          >
        </div>
      </div>
    </div>

    <!-- Command palette shortcut hint -->
    <KbdHint contents="?" :show="props.showHint" :inline="true" />
  </div>
</template>

<style scoped>
input[type="search"]::-webkit-search-decoration,
input[type="search"]::-webkit-search-cancel-button,
input[type="search"]::-webkit-search-results-decoration {
  display: none;
}
</style>
