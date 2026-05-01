/// <reference types="vite/client" />

declare const DEBUG: boolean;

declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent;
  export default component;
}

declare module "vue-material-design-icons/*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<{
    size?: number | string;
    fillColor?: string;
    title?: string;
  }>;
  export default component;
}
