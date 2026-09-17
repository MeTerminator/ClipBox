import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { VitePWA } from "vite-plugin-pwa";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    VitePWA({
      registerType: "prompt",
      includeAssets: ["favicon.svg", "pwa-192.png", "pwa-512.png"],
      manifest: {
        name: "ClipBox",
        short_name: "ClipBox",
        description: "轻量的临时文件、文本与共享剪贴板服务",
        theme_color: "#863bff",
        background_color: "#ffffff",
        display: "standalone",
        start_url: "/",
        icons: [
          { src: "/pwa-192.png", sizes: "192x192", type: "image/png" },
          { src: "/pwa-512.png", sizes: "512x512", type: "image/png" },
          { src: "/pwa-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
      workbox: {
        navigateFallback: "/index.html",
        cleanupOutdatedCaches: true,
        globPatterns: ["**/*.{js,css,html,svg,woff2}"],
        runtimeCaching: [
          {
            urlPattern: ({ url, sameOrigin }) => sameOrigin && !url.pathname.startsWith("/api/") && !url.pathname.startsWith("/clip/") && !url.pathname.startsWith("/file/") && !url.pathname.startsWith("/text/"),
            handler: "NetworkFirst",
            options: { cacheName: "clipbox-pages", networkTimeoutSeconds: 3 },
          },
        ],
      },
    }),
  ],
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
