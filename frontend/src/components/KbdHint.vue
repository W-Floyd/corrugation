<script setup lang="ts" name="KbdHint">
defineProps<{
  contents: string;
  show: boolean;
  inline?: boolean;
  center?: boolean;
}>();
</script>

<template>
  <kbd
    v-if="show"
    :class="
      inline
        ? 'pointer-events-none shrink-0 rounded bg-gray-800 px-1 font-sans text-[9px] leading-3.5 text-white shadow'
        : center
          ? 'pointer-events-none absolute -top-2 left-1/2 -translate-x-1/2 rounded bg-gray-800 px-1 font-sans text-[9px] leading-3.5 text-white shadow'
          : 'pointer-events-none absolute -top-2 -right-1 rounded bg-gray-800 px-1 font-sans text-[9px] leading-3.5 text-white shadow'
    "
    >{{ contents }}</kbd
  >
</template>
