import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "../stores/auth";

let configFetched = false;

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: () => import("../views/LoginView.vue"),
    },
    {
      path: "/callback",
      name: "callback",
      component: () => import("../views/CallbackView.vue"),
    },
    {
      path: "/settings",
      name: "settings",
      component: () => import("../views/SettingsView.vue"),
    },
  ],
});

router.beforeEach(async (to) => {
  const authStore = useAuthStore();
  DEBUG && console.log("[router] beforeEach to:", to.name, to.fullPath);

  if (!configFetched) {
    DEBUG && console.log("[router] fetching auth config");
    await authStore.fetchConfig();
    await authStore.fetchMe();
    configFetched = true;
  }

  const token = authStore.token;
  let authEnabled = authStore.authConfig.enabled;
  DEBUG && console.log("[router] token:", !!token, "authEnabled:", authEnabled);

  if (to.name === "callback") {
    DEBUG && console.log("[router] allowing callback route");
    return;
  }
  if (to.name === "login" && token) {
    DEBUG && console.log("[router] already authed, redirecting to /");
    return { path: "/" };
  }
  if (authEnabled && !token && to.name !== "login") {
    DEBUG && console.log("[router] not authed, redirecting to login");
    return { name: "login" };
  }
});

export default router;
