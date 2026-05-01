<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useRecordsStore } from "@/stores/records";
import { useAuthStore } from "@/stores/auth";
import LoginView from "@/views/LoginView.vue";
import SettingsView from "@/views/SettingsView.vue";
import RecordsView from "@/views/RecordsView.vue";

const routerReady = ref(false);
const router = useRouter();
const route = useRoute();
const recordsStore = useRecordsStore();
const authStore = useAuthStore();

onMounted(() => {
  router.isReady().then(() => {
    routerReady.value = true;
    if (DEBUG)
      console.log(
        "[app] router ready, route:",
        route.name,
        "token:",
        !!localStorage.getItem("auth_token"),
      );
    if (route.name !== "callback") {
      recordsStore.connectWS();
      authStore.fetchConfig();
    }
  });
});
</script>

<template>
  <template v-if="routerReady">
    <PrimeVueToast />

    <LoginView v-if="route.name === 'login'" />

    <RouterView v-else-if="route.name === 'callback'" />

    <SettingsView v-else-if="route.name === 'settings'" />

    <RecordsView v-else />
  </template>
</template>
