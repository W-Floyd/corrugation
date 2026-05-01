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
import { useToastsStore } from "@/stores/toasts";

const props = defineProps<{ showHint?: boolean }>();

const recordsStore = useRecordsStore();
const toastsStore = useToastsStore();

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
    <div class="flex flex-row flex-wrap items-center gap-2 mb-4">
        <!-- Search icon -->
        <div class="text-gray-500 dark:text-gray-400">
            <MagnifyIcon :size="24" />
        </div>

        <!-- Filter world checkbox -->
        <div class="flex items-center">
            <label class="relative flex items-center cursor-pointer" title="Only search in current record">
                <input type="checkbox" v-model="recordsStore.filterworld"
                    class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" @change="onWorldChange" />
                <EarthIcon class="relative ml-1 text-sm text-gray-600 dark:text-gray-400" :size="16" />
                <KbdHint contents="G" :show="props.showHint" :center="true" />
            </label>
        </div>

        <!-- Text embedding toggle -->
        <div class="flex items-center flex-shrink-0">
            <label class="relative flex items-center cursor-pointer" title="Use text embeddings in search">
                <input type="checkbox" v-model="recordsStore.searchTextEmbedded"
                    class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                <TextBoxSearchIcon class="relative ml-1 text-sm text-gray-600 dark:text-gray-400" :size="16" />
                <KbdHint contents="W" :show="props.showHint" :center="true" />
            </label>
        </div>

        <!-- String matching toggle -->
        <div class="flex items-center">
            <label class="relative flex items-center cursor-pointer" title="Use substring matching in search">
                <input type="checkbox" v-model="recordsStore.searchTextSubstring"
                    class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                <TextSearchIcon class="relative ml-1 text-sm text-gray-600 dark:text-gray-400" :size="16" />
                <KbdHint contents="T" :show="props.showHint" :center="true" />
            </label>
        </div>

        <!-- Image embedding toggle -->
        <div class="flex items-center">
            <label class="relative flex items-center cursor-pointer" title="Use image embeddings in search">
                <input type="checkbox" v-model="recordsStore.searchImage"
                    class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
                <ImageSearchIcon class="relative ml-1 text-sm text-gray-600 dark:text-gray-400" :size="16" />
                <KbdHint contents="I" :show="props.showHint" :center="true" />
            </label>
        </div>

        <!-- Search input -->
        <div class="relative flex-1 min-w-xs">
            <input ref="searchInputEl" v-model="recordsStore.searchtextpredebounce" @input="handleSearchInput"
                @keydown.enter.stop="searchInputEl?.blur()" @keydown.esc.stop="searchInputEl?.blur()"
                placeholder="Search for a record..." type="search"
                class="w-full px-4 py-2 rounded-full bg-white ring-1 ring-gray-300 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-800 dark:ring-gray-600 dark:text-white pr-14" />
            <kbd v-if="props.showHint"
                class="absolute right-3 top-1/2 -translate-y-1/2 text-[9px] font-sans bg-gray-800 text-white rounded px-1 leading-3.5 pointer-events-none shadow">/
            </kbd>
            <button v-if="recordsStore.searchtext" @click="resetSearch" type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 h-6 w-6 flex items-center justify-center rounded-full bg-gray-100 hover:bg-gray-200 text-gray-600 dark:bg-gray-700 dark:hover:bg-gray-600 dark:text-gray-300"
                title="Clear search">
                <CloseIcon :size="14" />
            </button>
            <!-- Image search indicator badge -->
            <div v-if="recordsStore.apiSearchResults.length > 0 && recordsStore.searchtext === '🔍'"
                class="absolute right-10 top-1/2 -translate-y-1/2 flex items-center gap-1">
                <div
                    class="flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/20 px-2 py-1 rounded-full">
                    <ImageSearchIcon :size="12" />
                    <span class="font-semibold">{{ recordsStore.apiSearchResults.length }}</span>
                    <span class="text-gray-500 dark:text-gray-400 text-[10px]">similar</span>
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
