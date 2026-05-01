<script setup lang="ts" name="BreadcrumbNav">
import { computed, ref } from "vue";
import { useRecordsStore } from "@/stores/records";
import { useToastsStore } from "@/stores/toasts";
import { api } from "@/api";

const recordsStore = useRecordsStore();
const toastsStore = useToastsStore();

const emit = defineEmits<{ openNewRecord: [] }>();

const locationTree = computed(() =>
    recordsStore.locationtree.map((id: number) => ({
        id,
        name: recordsStore.readname(id),
    })),
);

const navigateTo = async (recordId: number): Promise<void> => {
    await recordsStore.setCurrentRecord(recordId);
};

const dragOverId = ref<number | null>(null);

const handleDragOver = (e: DragEvent, id: number): void => {
    if (!e.dataTransfer?.types.includes("recordid")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    dragOverId.value = id;
};

const handleDragLeave = (e: DragEvent): void => {
    if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
        dragOverId.value = null;
    }
};

const handleDrop = async (e: DragEvent, targetId: number): Promise<void> => {
    dragOverId.value = null;
    const raw = e.dataTransfer?.getData("recordId");
    if (!raw) return;
    const recordId = parseInt(raw, 10);
    if (isNaN(recordId) || recordId === targetId) return;
    try {
        await api.moveRecord(recordId, targetId);
        await recordsStore.reload();
        toastsStore.add("Record moved", "info");
    } catch {
        toastsStore.add("Failed to move record");
    }
};
</script>

<template>
    <nav class="w-full">
        <ol class="flex flex-wrap items-center gap-x-1">
            <template v-for="(n, index) in locationTree" :key="n.id">
                <li>
                    <a @click="navigateTo(n.id)"
                        @dragover="handleDragOver($event, n.id)"
                        @dragleave="handleDragLeave"
                        @drop="handleDrop($event, n.id)"
                        :class="[
                            'text-blue-600 no-underline cursor-pointer dark:text-sky-400 dark:hover:text-sky-300 hover:text-blue-700 hover:underline px-1 rounded transition-colors',
                            dragOverId === n.id ? 'ring-2 ring-green-500 shadow shadow-green-200 dark:shadow-green-900 bg-green-50 dark:bg-green-900/20' : '',
                        ]"
                        :title="`Go to record ${n.id}`">
                        {{ n.name }}
                    </a>
                </li>

                <li v-if="index < locationTree.length - 1" aria-hidden="true">
                    <span class="text-gray-400">/</span>
                </li>
            </template>
        </ol>
    </nav>
</template>
