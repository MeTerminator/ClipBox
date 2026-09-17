<template>
  <Toaster
    position="top-right"
    :theme="isLight ? 'light' : 'dark'"
    :offset="{ top: 72, right: 16, bottom: 16, left: 16 }"
    :mobile-offset="{ top: 64, right: 12, bottom: 12, left: 'auto' }"
    :visible-toasts="1"
    :expand="false"
    :close-button="true"
    :duration="3600"
  />
  <div v-if="draggingFile || uploadStage" class="global-drop-overlay">
    <UploadCloudIcon />
    <strong v-if="uploadStage">{{
      $t("clip.uploading", { progress: uploadProgress })
    }}</strong>
    <strong v-else>{{
      roomFileDropHandler ? $t("rooms.dropToRoom") : $t("clip.dropToShare")
    }}</strong>
    <span v-if="!uploadStage">{{ $t("clip.dropRelease") }}</span>
  </div>
  <header class="sticky top-0 z-50 w-full bg-background border-b border-border">
    <div
      class="container flex h-14 max-w-screen-md items-center justify-between py-0"
    >
      <router-link
        :to="{ name: 'home' }"
        class="flex items-center gap-2 font-bold text-lg text-foreground hover:text-foreground/80 transition-colors"
      >
        <span>ClipBox</span>
      </router-link>
      <div class="flex items-center gap-4">
        <Select v-model="locale" @update:modelValue="saveLang">
          <SelectTrigger
            class="w-[110px] h-9 border-border bg-transparent focus:ring-1 focus:ring-foreground"
          >
            <SelectValue placeholder="Language" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="en">English</SelectItem>
            <SelectItem value="zh">简体中文</SelectItem>
          </SelectContent>
        </Select>

        <Button
          variant="outline"
          size="icon"
          @click="toggleTheme"
          class="h-9 w-9 border-border bg-transparent hover:bg-foreground hover:text-background transition-colors"
        >
          <Sun v-if="isLight" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </header>

  <main class="container max-w-screen-md py-8 flex-1">
    <router-view />
  </main>

  <footer class="bg-background border-t border-border py-6 mt-12">
    <div
      class="container max-w-screen-md flex flex-col items-center justify-between gap-4"
    >
      <p class="text-center text-sm leading-loose text-muted-foreground">
        &copy; {{ new Date().getFullYear() }}
        <a
          href="https://github.com/MeTerminator/ClipBox"
          target="_blank"
          class="font-medium underline underline-offset-4 hover:text-foreground"
        >
          ClipBox
        </a>
      </p>
    </div>
  </footer>
  <FileDetailModal
    :open="dropResultOpen"
    :item="dropResult"
    @close="dropResultOpen = false"
    @copied="toast.success(t('clip.copied'))"
  />
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { Toaster } from "@/components/ui/sonner";
import { Button } from "@/components/ui/button";
import FileDetailModal from "@/components/FileDetailModal.vue";
import { roomFileDropHandler } from "@/composables/fileDropTarget";
import { useFileUpload } from "@/composables/useFileUpload";
import { Moon, Sun, UploadCloudIcon } from "lucide-vue-next";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const { locale, t } = useI18n();
const isLight = ref(false);
const draggingFile = ref(false);
const dropResultOpen = ref(false);
const dropResult = ref(null);
const { uploadStage, uploadProgress, uploadFile, resetUploadProgress } =
  useFileUpload();
let dragDepth = 0;

onMounted(() => {
  const theme = localStorage.getItem("theme") || "dark";
  isLight.value = theme === "light";
  if (theme === "dark") {
    document.documentElement.classList.add("dark");
  } else {
    document.documentElement.classList.remove("dark");
  }
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

const containsFiles = (event) =>
  Array.from(event.dataTransfer?.types || []).includes("Files") ||
  Boolean(event.dataTransfer?.files?.length);
const handleDragEnter = (event) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  dragDepth += 1;
  draggingFile.value = true;
};
const handleDragOver = (event) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
};
const handleDragLeave = (event) => {
  if (!draggingFile.value) return;
  if (event.relatedTarget === null) dragDepth = 1;
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) draggingFile.value = false;
};
const handleDrop = async (event) => {
  if (!containsFiles(event)) return;
  event.preventDefault();
  dragDepth = 0;
  draggingFile.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (!file) return;
  if (roomFileDropHandler.value) {
    await roomFileDropHandler.value(file);
    return;
  }
  try {
    const count = 1000;
    const expire = 86400;
    const result = await uploadFile(file, { count, expire });
    const expiresAt = new Date(Date.now() + expire * 1000).toISOString();
    const item = {
      code: result.code,
      type: "file",
      filename: file.name,
      size: file.size,
      expiresAt,
      remainingCount: count,
      maxCount: count,
    };
    const history = JSON.parse(localStorage.getItem("clipHistory") || "[]");
    localStorage.setItem(
      "clipHistory",
      JSON.stringify(
        [item, ...history.filter((entry) => entry.code !== item.code)].slice(
          0,
          20,
        ),
      ),
    );
    dropResult.value = item;
    dropResultOpen.value = true;
  } catch (error) {
    toast.error(error.message || t("common.unexpectedError"));
  } finally {
    resetUploadProgress();
  }
};

const toggleTheme = () => {
  isLight.value = !isLight.value;
  if (isLight.value) {
    document.documentElement.classList.remove("dark");
    localStorage.setItem("theme", "light");
  } else {
    document.documentElement.classList.add("dark");
    localStorage.setItem("theme", "dark");
  }
};

const saveLang = (value) => {
  localStorage.setItem("lang", value);
};
</script>

<style scoped>
.global-drop-overlay {
  position: fixed;
  inset: 1rem;
  z-index: 100;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 0.65rem;
  border: 2px dashed hsl(var(--foreground) / 0.7);
  border-radius: 1.25rem;
  background: hsl(var(--background) / 0.94);
  backdrop-filter: blur(12px);
  pointer-events: none;
}

.global-drop-overlay svg {
  width: 2.5rem;
  height: 2.5rem;
}
.global-drop-overlay strong {
  font-size: 1.2rem;
}
.global-drop-overlay span {
  color: hsl(var(--muted-foreground));
  font-size: 0.85rem;
}
</style>
