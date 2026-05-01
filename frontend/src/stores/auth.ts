import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { useToast } from "@/utils/toast";
import { DEFAULT_TOAST_LIFE } from "./constants";

export interface AuthConfig {
  enabled: boolean;
  authorizationEndpoint?: string;
  tokenEndpoint?: string;
  clientId?: string;
}

function base64urlEncode(buf: Uint8Array): string {
  return btoa(String.fromCharCode(...buf))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
}

async function generateCodeVerifier(): Promise<string> {
  const arr = new Uint8Array(32);
  crypto.getRandomValues(arr);
  return base64urlEncode(arr);
}

async function generateCodeChallenge(verifier: string): Promise<string> {
  const data = new TextEncoder().encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return base64urlEncode(new Uint8Array(digest));
}

export const useAuthStore = defineStore("auth", () => {
  const toast = useToast();
  const token = ref<string | null>(localStorage.getItem("auth_token"));
  const authConfig = ref<AuthConfig>({ enabled: false });
  const isAdmin = ref(false);
  const username = ref("");

  const isAuthenticated = computed(() => token.value !== null);

  if (DEBUG) {
    console.log(
      "[auth] store init, token in localStorage:",
      !!localStorage.getItem("auth_token"),
    );
  }
  if (token.value) {
    document.cookie = `auth_token=${token.value}; path=/; SameSite=Strict`;
  }

  function setToken(t: string) {
    if (DEBUG) console.log("[auth] setToken", t.slice(0, 20) + "…");
    token.value = t;
    localStorage.setItem("auth_token", t);
    document.cookie = `auth_token=${t}; path=/; SameSite=Strict`;
  }

  function clearToken() {
    if (DEBUG) console.log("[auth] clearToken");
    token.value = null;
    localStorage.removeItem("auth_token");
    document.cookie =
      "auth_token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT";
  }

  async function fetchMe(): Promise<void> {
    if (!token.value) return;
    try {
      const resp = await fetch("/api/me", {
        headers: { Authorization: `Bearer ${token.value}` },
      });
      if (resp.ok) {
        const data: {
          username: string;
          isAdmin: boolean;
          justBecameAdmin: boolean;
        } = await resp.json();
        username.value = data.username;
        isAdmin.value = data.isAdmin;
        if (data.justBecameAdmin) {
          toast.add({
            severity: "success",
            summary: "Admin Access",
            detail:
              "You are the first user — you have been granted admin access.",
            life: DEFAULT_TOAST_LIFE,
          });
          console.log(
            "[auth] first admin user detected - admin access granted",
          );
        }
      }
    } catch (e) {
      if (DEBUG) console.warn("[auth] fetchMe error:", e);
    }
  }

  async function fetchConfig(): Promise<void> {
    if (DEBUG) console.log("[auth] fetchConfig start");
    try {
      const resp = await fetch("/api/auth/config");
      if (resp.ok) {
        authConfig.value = await resp.json();
        if (DEBUG) console.log("[auth] fetchConfig result:", authConfig.value);
      } else {
        if (DEBUG)
          console.warn("[auth] fetchConfig non-ok status:", resp.status);
      }
    } catch (e) {
      if (DEBUG) console.warn("[auth] fetchConfig error:", e);
    }
  }

  async function startLogin(): Promise<void> {
    const cfg = authConfig.value;
    if (DEBUG) console.log("[auth] startLogin, cfg:", cfg);
    if (!cfg.enabled || !cfg.authorizationEndpoint || !cfg.clientId) {
      console.error("[auth] startLogin aborted — config incomplete", cfg);
      return;
    }

    const verifier = await generateCodeVerifier();
    const challenge = await generateCodeChallenge(verifier);
    const state = base64urlEncode(crypto.getRandomValues(new Uint8Array(16)));

    sessionStorage.setItem("pkce_verifier", verifier);
    sessionStorage.setItem("pkce_state", state);
    if (DEBUG) console.log("[auth] startLogin PKCE stored, state:", state);

    const redirectUri = window.location.origin + "/callback";
    const params = new URLSearchParams({
      response_type: "code",
      client_id: cfg.clientId,
      redirect_uri: redirectUri,
      scope: "openid profile email",
      state,
      code_challenge: challenge,
      code_challenge_method: "S256",
    });

    const url = `${cfg.authorizationEndpoint}?${params}`;
    if (DEBUG) console.log("[auth] redirecting to:", url);
    window.location.href = url;
  }

  async function localLogin(loginUsername: string): Promise<void> {
    const response = await fetch("/api/auth/local/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: loginUsername }),
    });
    if (!response.ok) {
      const error = await response.text();
      const message = error || "Local login failed";
      clearToken();
      toast.add({
        severity: "error",
        summary: "Login Failed",
        detail: message,
        life: DEFAULT_TOAST_LIFE,
      });
      throw new Error(message);
    }
    const result = await response.json();
    setToken(result.username);
    await fetchMe();
  }

  async function handleCallback(code: string, state: string): Promise<boolean> {
    if (DEBUG) {
      console.log(
        "[auth] handleCallback start, code:",
        code.slice(0, 8) + "…",
        "state:",
        state,
      );
    }

    const storedState = sessionStorage.getItem("pkce_state");
    const verifier = sessionStorage.getItem("pkce_verifier");
    if (DEBUG) {
      console.log(
        "[auth] sessionStorage — storedState:",
        storedState,
        "hasVerifier:",
        !!verifier,
      );
    }

    sessionStorage.removeItem("pkce_state");
    sessionStorage.removeItem("pkce_verifier");

    if (state !== storedState || !verifier) {
      console.error("[auth] PKCE mismatch", {
        state,
        storedState,
        hasVerifier: !!verifier,
      });
      return false;
    }

    const cfg = authConfig.value;
    if (DEBUG) console.log("[auth] authConfig at callback time:", cfg);
    if (!cfg.tokenEndpoint || !cfg.clientId) {
      console.error("[auth] config missing tokenEndpoint/clientId", cfg);
      return false;
    }

    const redirectUri = window.location.origin + "/callback";
    const body = new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: redirectUri,
      client_id: cfg.clientId,
      code_verifier: verifier,
    });

    if (DEBUG) {
      console.log("[auth] posting to tokenEndpoint:", cfg.tokenEndpoint);
    }
    const resp = await fetch(cfg.tokenEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });

    if (DEBUG) console.log("[auth] token response status:", resp.status);
    if (!resp.ok) {
      const text = await resp.text();
      console.error("[auth] token exchange failed:", resp.status, text);
      return false;
    }

    const data = await resp.json();
    if (DEBUG) console.log("[auth] token response keys:", Object.keys(data));
    const jwt = data.access_token ?? data.id_token;
    if (!jwt) {
      console.error("[auth] no token in response", data);
      return false;
    }

    setToken(jwt);
    await fetchMe();
    return true;
  }

  return {
    token,
    authConfig,
    isAdmin,
    username,
    isAuthenticated,
    setToken,
    clearToken,
    fetchConfig,
    fetchMe,
    startLogin,
    handleCallback,
    localLogin,
  };
});
