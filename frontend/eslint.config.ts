import js from "@eslint/js";
import globals from "globals";
import tseslint from "typescript-eslint";
import pluginVue from "eslint-plugin-vue";
import css from "@eslint/css";
import { defineConfig } from "eslint/config";
import eslintConfigPrettier from "eslint-config-prettier";

export default defineConfig([
  {
    files: ["**/*.{js,mjs,cjs,ts,mts,cts,vue}"],
    plugins: {
      js,
    },
    extends: ["js/recommended", eslintConfigPrettier],
    languageOptions: {
      globals: { ...globals.browser, DEBUG: "readonly" },
    },
  },
  tseslint.configs.recommended,
  pluginVue.configs["flat/essential"],
  {
    files: ["**/*.vue"],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
    rules: {
      // TypeScript handles undef checks for TS-parsed Vue files
      "no-undef": "off",
    },
  },
  {
    files: ["*.config.js", "*.config.ts", "postcss.config.js"],
    languageOptions: {
      globals: { ...globals.node, ...globals.browser },
    },
  },
  {
    files: ["**/*.css"],
    plugins: {
      css,
    },
    language: "css/css",
    extends: ["css/recommended"],
  },
]);
