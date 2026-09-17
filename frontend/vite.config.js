import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import path from "path";

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: "../www",
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/clip": {
        target: "http://localhost:5328",
        changeOrigin: true,
      },
      "/file": {
        target: "http://localhost:5328",
        changeOrigin: true,
      },
      "/text": {
        target: "http://localhost:5328",
        changeOrigin: true,
      },
      "/api": { target: "http://localhost:5328", changeOrigin: true },
    },
  },
});
