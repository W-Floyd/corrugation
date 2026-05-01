<template>
  <div
    class="flex min-h-screen items-center justify-center bg-gray-100 dark:bg-gray-900"
  >
    <p class="text-sm text-gray-600 dark:text-gray-400">{{ status }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useRecordsStore } from "../stores/records";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const recordsStore = useRecordsStore();
const status = ref("Completing sign-in…");

DEBUG && console.log("[callback] component setup");

onMounted(async () => {
  DEBUG && console.log("[callback] onMounted, query:", route.query);
  const code = route.query.code as string | undefined;
  const state = route.query.state as string | undefined;

  if (!code || !state) {
    console.error("[callback] missing code or state");
    status.value = "Invalid callback — missing code or state.";
    return;
  }

  let ok = false;
  try {
    ok = await authStore.handleCallback(code, state);
  } catch (e) {
    console.error("[callback] handleCallback threw:", e);
    status.value = `Sign-in error: ${e}`;
    return;
  }

  DEBUG && console.log("[callback] handleCallback result:", ok);
  if (!ok) {
    status.value = "Sign-in failed — check console for details.";
    setTimeout(() => router.push({ name: "login" }), 4000);
    return;
  }

  DEBUG && console.log("[callback] token set, loading state");
  await recordsStore.reload();
  DEBUG && console.log("[callback] state loaded, navigating to /");
  router.push({ path: "/" });
});
</script>
