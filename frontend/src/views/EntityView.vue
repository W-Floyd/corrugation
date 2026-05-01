<script setup lang="ts">
import { ref, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useEntitiesStore } from "@/stores/entities";
import { useCameraStore } from "@/stores/camera";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";
import type { Entity } from "@/api/types";

const route = useRoute();
const router = useRouter();
const entitiesStore = useEntitiesStore();
const cameraStore = useCameraStore();
const toastsStore = useToastsStore();
const authStore = useAuthStore();

const isLoading = computed(() => entitiesStore.isLoading);

// Watch for route changes to update current entity
import { watch } from "vue";
import { onMounted } from "vue";
import { useAuthStore } from "@/stores/auth";

onMounted(() => {
    entitiesStore.connectWS();
});

watch(
    () => route.query.entity,
    async (newId) => {
        await entitiesStore.setCurrentEntity(
            newId ? parseInt(newId as string, 10) : 0,
        );
    },
    { immediate: true },
);

// Dialog state
const newEntityVisible = ref(false);
const moveEntityVisible = ref(false);
const newEntityTemp = ref({ name: "", description: "" });
const moveEntitySearch = ref("");
const moveEntityTarget = ref<number>(0);

// Filtered entities for move dialog
const filteredEntities = computed(() => {
    if (!moveEntitySearch.value.trim()) {
        return Object.values(entitiesStore.entityMap).filter(
            (e) => e.id !== 0 && e.location !== 0,
        );
    }
    const term = moveEntitySearch.value.toLowerCase();
    return Object.values(entitiesStore.entityMap).filter(
        (e) =>
            e.id !== 0 &&
            e.location !== 0 &&
            (e.name?.toLowerCase().includes(term) ||
                e.description?.toLowerCase().includes(term) ||
                e.id.toString().includes(term)),
    );
});

const handleNewEntitySubmit = async (): Promise<void> => {
    if (!newEntityTemp.value.name || !newEntityTemp.value.name.trim()) return;
    try {
        const record = await api.createRecord({
            Title: newEntityTemp.value.name,
            Description: newEntityTemp.value.description || null,
            ParentID: entitiesStore.currentEntity || undefined,
        });
        const entityId = record.ID;
        await entitiesStore.reload();
        await entitiesStore.setCurrentEntity(entityId);
        newEntityVisible.value = false;
    } catch (error) {
        console.error("Failed to create entity:", error);
    }
};

function openNewEntityDialog(_entityId?: number): void {
    newEntityVisible.value = true;
}

const handleMoveEntitySubmit = async (): Promise<void> => {
    if (!moveEntityTarget.value || moveEntityTarget.value === 0) return;
    try {
        await api.moveRecord(
            Number(route.query.entity ?? entitiesStore.currentEntity),
            moveEntityTarget.value,
        );
        await entitiesStore.reload();
        moveEntityVisible.value = false;
    } catch (error) {
        console.error("Failed to move entity:", error);
    }
};
</script>

<template>
    <div class="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-white">
        <!-- Header with breadcrumbs -->
        <div class="container mx-auto pt-4 px-4">
            <nav class="w-full flex items-center justify-between">
                <ol class="flex flex-wrap list-reset">
                    <template v-for="(n, index) in entitiesStore.locationtree" :key="n">
                        <li>
                            <a @click="entitiesStore.setCurrentEntity(n)"
                                class="text-blue-600 no-underline cursor-pointer dark:text-sky-400 dark:hover:text-sky-300 hover:text-blue-700 hover:underline"
                                :title="`Go to entity ${n}`">
                                {{ entitiesStore.readname(n) }}
                            </a>
                        </li>

                        <li v-if="index < entitiesStore.locationtree.length - 1">
                            <span class="mx-2 text-gray-500">/</span>
                        </li>
                    </template>
                    <li>
                        <a @click="openNewEntityDialog(entitiesStore.currentEntity)"
                            class="text-blue-600 dark:text-sky-400 dark:hover:text-sky-300 hover:text-blue-700 cursor-pointer"
                            title="Create new entity">
                            +
                        </a>
                    </li>
                </ol>
                <router-link to="/settings"
                    class="text-sm text-blue-600 dark:text-sky-400 hover:text-blue-700 dark:hover:text-sky-300 hover:underline transition-colors shrink-0 ml-4">
                    Settings
                </router-link>
            </nav>

            <!-- Search bar -->
            <SearchBar />
        </div>

        <!-- Empty state or entity list -->
        <div class="container mx-auto px-4 mt-4">
            <!-- Empty state -->
            <div v-if="!entitiesStore.hasChildren(entitiesStore.currentEntity)">
                <p class="text-2xl text-gray-500/50">Empty</p>
            </div>

            <!-- Entity grid -->
            <div class="flex flex-wrap justify-center gap-4">
                <EntityCard v-for="entity in entitiesStore.load(
                    entitiesStore.currentEntity,
                    entitiesStore.searchtext,
                )" :key="entity.id" :entity="entity" />
            </div>
        </div>

        <!-- Dialogs -->
        <div v-if="newEntityVisible" class="fixed inset-0 overflow-y-auto z-50" role="dialog" aria-modal="true">
            <!-- Overlay -->
            <div class="fixed inset-0 bg-black bg-opacity-50 dark:bg-gray-900 dark:bg-opacity-50"
                @click="newEntityVisible = false"></div>

            <!-- Panel -->
            <div class="relative flex items-center justify-center min-h-screen p-4" @click.stop>
                <div
                    class="relative w-full max-w-2xl p-8 overflow-y-auto bg-white border border-gray-300 rounded-lg dark:bg-gray-800">
                    <!-- Title -->
                    <div class="grid grid-cols-2 w-fit space-x-2 items-baseline mb-4">
                        <h1 class="pb-4 text-3xl font-medium">
                            Create New Entity
                        </h1>
                        <h2 class="pb-4 text-2xl font-medium text-gray-500 dark:text-white/50">
                            ({{ entitiesStore.currentEntity }})
                        </h2>
                    </div>

                    <!-- Simple form for now -->
                    <div class="mb-4">
                        <input v-model="newEntityTemp.name" type="text" placeholder="Name"
                            class="w-full px-4 py-2 rounded bg-white dark:bg-gray-900 ring-1" autofocus />
                    </div>

                    <div class="mb-4">
                        <textarea v-model="newEntityTemp.description" placeholder="Description"
                            class="w-full px-4 py-2 rounded bg-white dark:bg-gray-900 ring-1" rows="3"></textarea>
                    </div>

                    <!-- Buttons -->
                    <div class="flex mt-8 space-x-2">
                        <button @click="handleNewEntitySubmit"
                            class="h-10 px-4 py-2 text-white bg-blue-500 rounded-full shadow hover:bg-blue-600">
                            Submit
                        </button>
                        <button @click="newEntityVisible = false"
                            class="h-10 px-4 py-2 text-white bg-red-500 rounded-full shadow hover:bg-red-600">
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Move Entity Dialog -->
        <div v-if="moveEntityVisible" class="fixed inset-0 overflow-y-auto z-50" role="dialog" aria-modal="true">
            <!-- Overlay -->
            <div class="fixed inset-0 bg-black bg-opacity-50 dark:bg-gray-900 dark:bg-opacity-50"
                @click="moveEntityVisible = false"></div>

            <!-- Panel -->
            <div class="relative flex items-center justify-center min-h-screen p-4" @click.stop>
                <div
                    class="relative w-full max-w-2xl p-8 overflow-y-auto bg-white border border-gray-300 rounded-lg dark:bg-gray-800">
                    <!-- Title -->
                    <h1 class="pb-4 text-3xl font-medium">Move Entity</h1>

                    <div class="mb-4">
                        <input v-model="moveEntitySearch" type="search" placeholder="Search for a location..."
                            class="w-full px-4 py-2 rounded-full bg-white ring-1 dark:bg-gray-900" />
                    </div>

                    <div class="mb-4">
                        <label for="location" class="block mb-2">Select location</label>
                        <select v-model="moveEntityTarget" id="location"
                            class="w-full px-4 py-2 rounded-full bg-white ring-1 dark:bg-gray-900">
                            <option value="" disabled>Select a location</option>
                            <option value="0">(0) World</option>
                            <template v-for="loc in filteredEntities" :key="loc.id">
                                <option v-if="!entitiesStore.hasChildren(loc.id)" :value="loc.id">
                                    ({{ loc.id }}) {{ loc.name || loc.id }}
                                </option>
                            </template>
                        </select>
                    </div>

                    <!-- Buttons -->
                    <div class="flex mt-8 space-x-2">
                        <button @click="handleMoveEntitySubmit"
                            class="h-10 px-4 py-2 text-white bg-blue-500 rounded-full shadow hover:bg-blue-600">
                            Move Here
                        </button>
                        <button @click="moveEntityVisible = false"
                            class="h-10 px-4 py-2 text-white bg-red-500 rounded-full shadow hover:bg-red-600">
                            Cancel
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped></style>
