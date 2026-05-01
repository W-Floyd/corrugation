<template>
  <img v-if="objectUrl" :src="objectUrl" :alt="alt" v-bind="$attrs" />
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from "vue";
import { apiFetch } from "@/api";

const props = defineProps<{
  artifactId: number;
  alt?: string;
}>();

interface CacheEntry {
  etag: string;
  objectUrl: string;
}

const cache = new Map<number, CacheEntry>();

const objectUrl = ref<string | null>(null);

async function load(id: number) {
  try {
    const cached = cache.get(id);
    const headers: HeadersInit = cached ? { "If-None-Match": cached.etag } : {};

    const response = await apiFetch(`/api/artifact/${id}`, { headers });

    if (response.status === 304 && cached) {
      objectUrl.value = cached.objectUrl;
      return;
    }

    const etag = response.headers.get("ETag");
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);

    if (etag) {
      if (cached) URL.revokeObjectURL(cached.objectUrl);
      cache.set(id, { etag, objectUrl: url });
    }

    objectUrl.value = url;
  } catch {
    objectUrl.value = null;
  }
}

watch(() => props.artifactId, load, { immediate: true });

onUnmounted(() => {
  if (
    objectUrl.value &&
    cache.get(props.artifactId)?.objectUrl !== objectUrl.value
  ) {
    URL.revokeObjectURL(objectUrl.value);
  }
});
</script>
