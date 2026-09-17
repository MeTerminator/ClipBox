<template>
  <Toaster position="top-right" :theme="isLight ? 'light' : 'dark'" rich-colors close-button />

  <div v-if="draggingFile || uploadStage" class="fixed inset-4 z-[100] grid place-content-center justify-items-center gap-3 rounded-xl border-2 border-dashed bg-background/95 backdrop-blur-sm">
    <UploadCloudIcon class="size-10 text-muted-foreground" />
    <p class="font-medium">
      {{ uploadStage ? t("clip.uploading", { progress: uploadProgress }) : roomFileDropHandler ? t("rooms.dropToRoom") : t("clip.dropToShare") }}
    </p>
    <p v-if="!uploadStage" class="text-sm text-muted-foreground">{{ t("clip.dropRelease") }}</p>
  </div>

  <header class="sticky top-0 z-50 border-b bg-background/95 backdrop-blur">
    <div class="mx-auto flex h-14 w-full max-w-5xl items-center justify-between px-4">
      <router-link :to="{ name: 'home' }" class="font-semibold">ClipBox</router-link>
      <div class="flex items-center gap-2">
        <Select v-model="locale" @update:model-value="saveLang">
          <SelectTrigger class="w-32"><SelectValue placeholder="Language" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="en">English</SelectItem>
            <SelectItem value="zh">简体中文</SelectItem>
          </SelectContent>
        </Select>
        <Button variant="outline" size="icon" :aria-label="isLight ? t('nav.themeDark') : t('nav.themeLight')" @click="toggleTheme">
          <Sun v-if="isLight" />
          <Moon v-else />
        </Button>
      </div>
    </div>
  </header>

  <main class="mx-auto w-full max-w-5xl flex-1 px-4 py-8"><router-view /></main>

  <footer class="border-t py-6 text-center text-sm text-muted-foreground">
    &copy; {{ new Date().getFullYear() }}
    <a href="https://github.com/MeTerminator/ClipBox" target="_blank" class="underline underline-offset-4">ClipBox</a>
  </footer>

  <FileDetailModal
    :open="dropResultOpen"
    :item="dropResult"
    @close="dropResultOpen = false"
    @copied="toast.success(t('clip.copied'))"
  />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { Moon, Sun, UploadCloudIcon } from "@lucide/vue";
import { Toaster } from "@/components/ui/sonner";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import FileDetailModal from "@/components/FileDetailModal.vue";
import { roomFileDropHandler } from "@/composables/fileDropTarget";
import { useFileUpload } from "@/composables/useFileUpload";
import type { ClipRecord } from "@/types";

const { locale, t } = useI18n();
const isLight = ref(true);
const draggingFile = ref(false);
const dropResultOpen = ref(false);
const dropResult = ref<ClipRecord | null>(null);
const { uploadStage, uploadProgress, uploadFile, resetUploadProgress } = useFileUpload();
let dragDepth = 0;

onMounted(() => {
  const theme = localStorage.getItem("theme") || "light";
  isLight.value = theme === "light";
  document.documentElement.classList.toggle("dark", !isLight.value);
  window.addEventListener("dragenter", handleDragEnter);
  window.addEventListener("dragover", handleDragOver);
  window.addEventListener("dragleave", handleDragLeave);
  window.addEventListener("drop", handleDrop);
});

onBeforeUnmount(() => {
  window.removeEventListener("dragenter", handleDragEnter);
  window.removeEventListener("dragover", handleDragOver);
  window.removeEventListener("dragleave", handleDragLeave);
  window.removeEventListener("drop", handleDrop);
});

const containsFiles = (event: DragEvent) =>
  Array.from(event.dataTransfer?.types || []).includes("Files") || Boolean(event.dataTransfer?.files.length);
const handleDragEnter = (event: DragEvent) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  dragDepth += 1;
  draggingFile.value = true;
};
const handleDragOver = (event: DragEvent) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
};
const handleDragLeave = (event: DragEvent) => {
  if (!draggingFile.value) return;
  if (event.relatedTarget === null) dragDepth = 1;
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) draggingFile.value = false;
};
const handleDrop = async (event: DragEvent) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  draggingFile.value = false;
  const file = event.dataTransfer?.files[0];
  if (!file) return;
  if (roomFileDropHandler.value) {
    await roomFileDropHandler.value(file);
    return;
  }
  try {
    const count = 1000;
    const expire = 86400;
    const result = await uploadFile(file, { count, expire });
    const item: ClipRecord = {
      code: result.code,
      type: "file",
      filename: file.name,
      size: file.size,
      expiresAt: new Date(Date.now() + expire * 1000).toISOString(),
      remainingCount: count,
      maxCount: count,
    };
    const history = JSON.parse(localStorage.getItem("clipHistory") || "[]") as ClipRecord[];
    localStorage.setItem("clipHistory", JSON.stringify([item, ...history.filter((entry) => entry.code !== item.code)].slice(0, 20)));
    dropResult.value = item;
    dropResultOpen.value = true;
  } catch (error) {
    toast.error(error instanceof Error ? error.message : t("common.unexpectedError"));
  } finally {
    resetUploadProgress();
  }
};

const toggleTheme = () => {
  isLight.value = !isLight.value;
  document.documentElement.classList.toggle("dark", !isLight.value);
  localStorage.setItem("theme", isLight.value ? "light" : "dark");
};
const saveLang = (value: unknown) => localStorage.setItem("lang", String(value));
</script>
