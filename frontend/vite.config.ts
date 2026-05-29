import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import vueDevTools from "vite-plugin-vue-devtools";
import faviconPlugin from "vite-plugin-favicon-generator";
import basicSsl from "@vitejs/plugin-basic-ssl";

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [
    tailwindcss(),
    vue(),
    vueDevTools(),
    basicSsl(),
    faviconPlugin({
      appName: "Corrugation",
      appShortName: "Corrugation",
      appDescription: "Corrugation",
      developerName: "William Floyd",
      source: "src/assets/favicon.svg", // Source favicon image
      icons: {
        android: true, // Create Android homescreen icon
        appleIcon: true, // Create Apple touch icons
        appleStartup: false, // Create Apple startup images
        favicons: true, // Create regular favicons
        windows: false, // Create Windows 8 tile icons
        yandex: false, // Create Yandex browser icon
      },
    }),
  ],
  define: {
    DEBUG: mode !== "production",
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: "../dist",
    emptyOutDir: true,
    sourcemap: mode === "development",
  },
  server: {
    host: "0.0.0.0",
    https: {},
    proxy: {
      "/api": {
        target: "http://localhost:8083",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:8083",
        ws: true,
      },
    },
  },
}));
