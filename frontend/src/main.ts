import { createApp } from "vue";
import "vue-sonner/style.css";
import "./style.css";
import App from "./App.vue";
import router from "./router";
import i18n from "./i18n";
import { backendOrigin, initializeUploadRoute } from "./lib/backend";
import { setDesktopBackendOrigin } from "./lib/desktop";
import { registerSW } from "virtual:pwa-register";

if (!("__TAURI_INTERNALS__" in window)) {
  const updateSW = registerSW({
    immediate: true,
    onNeedRefresh() {
      if (window.confirm("ClipBox 已更新，是否立即刷新页面？")) void updateSW(true);
    },
  });
}

void setDesktopBackendOrigin(backendOrigin).catch(() => undefined);
void initializeUploadRoute().then(setDesktopBackendOrigin).catch(() => undefined);
createApp(App).use(router).use(i18n).mount("#app");
