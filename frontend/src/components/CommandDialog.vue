<script setup lang="ts" name="CommandDialog">
import { ref, computed } from "vue";
import CloseIcon from "vue-material-design-icons/Close.vue";

defineProps<{ visible: boolean }>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();

const search = ref("");

const commands = [
  { label: "Select next tile", shortcut: "↓ / →" },
  { label: "Select previous tile", shortcut: "↑ / ←" },
  { label: "Descend into selected record", shortcut: "Enter" },
  { label: "Ascend to parent", shortcut: "⌫" },
  { label: "Edit selected record", shortcut: "E" },
  { label: "Delete selected record", shortcut: "D" },
  { label: "Quick capture on selected", shortcut: "P" },
  { label: "Quick capture in location", shortcut: "C" },
  { label: "Quick capture new child", shortcut: "⇧C" },
  { label: "New record in location", shortcut: "N" },
  { label: "New record under selected", shortcut: "⇧N" },
  { label: "Move selected record", shortcut: "M" },
  { label: "Toggle image embedding search", shortcut: "I" },
  { label: "Toggle text embedding search", shortcut: "W" },
  { label: "Toggle string matching search", shortcut: "T" },
  { label: "Toggle global search", shortcut: "G" },
  { label: "Search", shortcut: "/" },
  { label: "Command palette", shortcut: "?" },
];

const filtered = computed(() => {
  const term = search.value.toLowerCase().trim();
  if (!term) return commands;
  return commands.filter(
    (c) =>
      c.label.toLowerCase().includes(term) ||
      c.shortcut.toLowerCase().includes(term),
  );
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="fixed inset-0 z-50 overflow-y-auto"
      role="dialog"
      aria-modal="true"
    >
      <div
        class="fixed inset-0 bg-black/40"
        @click="emit('update:visible', false)"
      ></div>
      <div
        class="relative flex min-h-screen items-start justify-center p-4 pt-[20vh]"
        @click.stop
      >
        <div
          class="w-full max-w-md overflow-hidden rounded-xl bg-white shadow-2xl ring-1 ring-gray-200 dark:bg-gray-800 dark:ring-gray-700"
        >
          <!-- Search input -->
          <div
            class="flex items-center gap-2 border-b px-4 py-3 dark:border-gray-700"
          >
            <input
              v-model="search"
              type="text"
              placeholder="Search commands..."
              class="flex-1 bg-transparent text-gray-900 placeholder-gray-400 outline-none dark:text-white"
              autofocus
              @keydown.escape="emit('update:visible', false)"
            />
            <button
              @click="emit('update:visible', false)"
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              <CloseIcon :size="18" />
            </button>
          </div>

          <!-- Command list -->
          <ul class="max-h-80 overflow-y-auto py-2">
            <li
              v-for="cmd in filtered"
              :key="cmd.label"
              class="flex items-center justify-between px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700"
            >
              <span class="text-sm text-gray-700 dark:text-gray-300">{{
                cmd.label
              }}</span>
              <kbd
                class="rounded border border-gray-200 bg-gray-100 px-1.5 py-0.5 font-mono text-xs text-gray-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-400"
                >{{ cmd.shortcut }}</kbd
              >
            </li>
            <li
              v-if="filtered.length === 0"
              class="px-4 py-3 text-sm text-gray-400"
            >
              No commands found
            </li>
          </ul>
        </div>
      </div>
    </div>
  </Teleport>
</template>
