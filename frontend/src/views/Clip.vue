<template>
  <Card class="mx-auto max-w-2xl p-6">
    <!-- Header Back Link -->
    <div class="mb-6">
      <Button as-child variant="ghost" class="-ml-3">
        <router-link :to="{ name: 'home' }"><ArrowLeftIcon />{{ $t("clip.backToHome") }}</router-link>
      </Button>
    </div>

    <!-- Redirect notice banner -->
    <Alert v-if="route.query.redirect" class="mb-6">
      <InfoIcon />
      <AlertDescription>{{ $t("clip.redirectNotice", { domain: getDomain(route.query.redirect) }) }}</AlertDescription>
    </Alert>

    <!-- Tabs Navigation -->
    <Tabs v-model="activeTab" class="w-full">
      <TabsList class="grid w-full grid-cols-4 mb-6">
        <TabsTrigger value="text">{{ $t("clip.text") }}</TabsTrigger>
        <TabsTrigger value="link">{{ $t("clip.link") }}</TabsTrigger>
        <TabsTrigger value="file">{{ $t("clip.file") }}</TabsTrigger>
        <TabsTrigger value="history">{{ $t("clip.history") }}</TabsTrigger>
      </TabsList>
    </Tabs>

    <!-- Form Section -->
    <div v-if="activeTab !== 'history'">
      <form @submit.prevent="submitClip" class="space-y-6">
        <!-- Text Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'text'">
          <Label class="text-sm font-semibold text-muted-foreground">{{
            $t("clip.contentLabel")
          }}</Label>
          <Textarea v-model="formData.content" class="min-h-36" required />
        </div>

        <!-- Link Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'link'">
          <Label class="text-sm font-semibold text-muted-foreground">{{
            $t("clip.linkUrl")
          }}</Label>
          <Input v-model="formData.content" type="url" required />
        </div>

        <!-- File Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'file'">
          <Label class="text-sm font-semibold text-muted-foreground">{{
            $t("clip.selectFile")
          }}</Label>
          <div
            class="rounded-lg p-8 text-center cursor-pointer transition-colors duration-200 flex flex-col items-center justify-center min-h-[150px] border-border hover:border-foreground border-dashed border"
            @click="fileInput?.click()"
          >
            <input
              type="file"
              ref="fileInput"
              @change="handleFileSelect"
              class="hidden"
            />
            <div class="space-y-2">
              <div class="flex justify-center text-muted-foreground">
                <UploadCloudIcon class="h-10 w-10 stroke-1" />
              </div>
              <div class="text-sm">
                <p v-if="!selectedFile" class="font-medium text-foreground">
                  {{ $t("clip.dropFile") }}
                </p>
                <p v-else class="font-bold text-foreground">
                  {{ selectedFile.name }} ({{ formatSize(selectedFile.size) }})
                </p>
              </div>
            </div>
          </div>
          <div v-if="uploadStage" class="space-y-2" aria-live="polite">
            <div class="flex justify-between text-xs text-muted-foreground">
              <span>
                {{
                  uploadStage === "hashing"
                    ? $t("clip.hashing", { progress: uploadProgress })
                    : $t("clip.uploading", { progress: uploadProgress })
                }}
              </span>
              <span>{{ uploadProgress }}%</span>
            </div>
            <Progress :model-value="uploadProgress" />
            <p v-if="resumedChunks" class="text-xs text-muted-foreground">
              {{ $t("clip.resuming", { count: resumedChunks }) }}
            </p>
          </div>
        </div>

        <!-- Access Options Row -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="flex flex-col gap-2">
            <Label class="text-sm font-semibold text-muted-foreground">{{
              $t("clip.countLabel")
            }}</Label>
            <Input v-model="formData.count" type="number" min="1" />
          </div>
          <div class="flex flex-col gap-2">
            <Label class="text-sm font-semibold text-muted-foreground">{{
              $t("clip.expireLabel")
            }}</Label>
            <div class="flex gap-2">
              <Input v-model="formData.expire" type="number" min="1" class="flex-1" />
              <Select v-model="formData.expireUnit">
                <SelectTrigger class="w-[110px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">{{
                    $t("clip.units.seconds")
                  }}</SelectItem>
                  <SelectItem value="60">{{
                    $t("clip.units.minutes")
                  }}</SelectItem>
                  <SelectItem value="3600">{{
                    $t("clip.units.hours")
                  }}</SelectItem>
                  <SelectItem value="86400">{{
                    $t("clip.units.days")
                  }}</SelectItem>
                  <SelectItem value="604800">{{
                    $t("clip.units.weeks")
                  }}</SelectItem>
                  <SelectItem value="31536000">{{
                    $t("clip.units.years")
                  }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>

        <!-- Submit Button -->
        <Button
          type="submit"
          class="w-full"
          size="lg"
          :disabled="loading"
        >
          {{
            loading
              ? "..."
              : activeTab === "file"
                ? $t("clip.upload")
                : $t("clip.create")
          }}
        </Button>
      </form>

      <!-- Redirect Confirmation Box -->
      <Alert v-if="pendingRedirect" class="mt-6">
        <InfoIcon />
        <AlertDescription class="space-y-3">
          <p>
          {{
            $t("clip.redirectMsg", { domain: getDomain(route.query.redirect) })
          }}
          </p>
          <div class="flex gap-3">
          <Button @click="doRedirect" size="sm" class="text-xs font-semibold">
            {{ $t("clip.redirectBtn") }}
          </Button>
          <Button
            @click="pendingRedirect = null"
            size="sm"
            variant="outline"
            class="text-xs"
          >
            {{ $t("clip.stayBtn") }}
          </Button>
          </div>
        </AlertDescription>
      </Alert>

      <!-- Error Message Box -->
      <Alert v-if="errorMsg" variant="destructive" class="mt-6">
        <AlertDescription>{{ $t("clip.error") }}{{ errorMsg }}</AlertDescription>
      </Alert>
    </div>

    <!-- History Tab Section -->
    <div v-else class="space-y-4">
      <div class="flex items-center justify-between pb-2">
        <h3 class="text-lg font-bold text-foreground">
          {{ $t("clip.history") }}
        </h3>
        <Button
          v-if="history.length"
          @click="clearHistory"
          variant="outline"
          size="sm"
          class="border-border bg-transparent hover:bg-destructive hover:text-destructive-foreground hover:border-destructive gap-1.5"
        >
          <Trash2Icon class="h-3.5 w-3.5" />
          {{ $t("clip.clearHistory") }}
        </Button>
      </div>

      <ul v-if="history.length" class="space-y-2">
        <li
          v-for="item in history"
          :key="item.code"
          class="flex items-center justify-between rounded-lg border p-3"
        >
          <div class="flex w-full items-center gap-3 overflow-hidden">
            <span class="text-muted-foreground shrink-0">
              <component :is="getIconComponent(item.type)" class="h-4 w-4" />
            </span>
            <button
              type="button"
              class="font-bold font-mono text-foreground hover:underline hover:text-foreground/80 flex items-center gap-1 shrink-0"
              @click="openHistoryItem(item)"
            >
              {{ item.code }}
              <ExternalLinkIcon class="h-3 w-3 text-muted-foreground" />
            </button>
            <span class="text-xs text-muted-foreground shrink-0"
              >({{ $t(`clip.${item.type}`) }})</span
            >
            <span
              v-if="item.filename"
              class="text-xs text-muted-foreground font-medium truncate max-w-[150px] sm:max-w-[300px]"
            >
              - {{ item.filename }}
            </span>
            <span
              v-if="item.remainingCount !== undefined"
              class="ml-auto shrink-0 text-xs text-muted-foreground"
            >
              {{ $t("home.remainingCount") }} {{ item.remainingCount }}
            </span>
          </div>
        </li>
      </ul>
      <div
        v-else
        class="text-center py-12 border border-dashed rounded-lg border-border/60 text-muted-foreground text-sm"
      >
        {{ $t("clip.noHistory") }}
      </div>
    </div>
  </Card>

  <FileDetailModal
    :open="detailModalOpen"
    :item="detailItem"
    @close="detailModalOpen = false"
    @copied="toast.success(t('clip.copied'))"
  />
</template>

<script setup lang="ts">
import { ref, onMounted, watch, type Component } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { Card } from "@/components/ui/card";
import FileDetailModal from "@/components/FileDetailModal.vue";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Progress } from "@/components/ui/progress";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "vue-sonner";
import { useFileUpload } from "@/composables/useFileUpload";
import { backendURL } from "@/lib/backend";
import type { ClipKind, ClipRecord } from "@/types";
import {
  ArrowLeftIcon,
  UploadCloudIcon,
  Trash2Icon,
  FileTextIcon,
  LinkIcon,
  FileIcon,
  ExternalLinkIcon,
  InfoIcon,
} from "@lucide/vue";

const route = useRoute();
const { t } = useI18n();

type CreateTab = "text" | "link" | "file" | "history";

const activeTab = ref<CreateTab>(route.query.tab === "file" ? "file" : "text");
const formData = ref({
  content: "",
  count: 1000,
  expire: 1,
  expireUnit: "86400",
});
const selectedFile = ref<File | null>(null);
const loading = ref(false);
const errorMsg = ref("");
const pendingRedirect = ref<string | null>(null);
const history = ref<ClipRecord[]>([]);
const detailModalOpen = ref(false);
const detailItem = ref<ClipRecord | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const {
  uploadStage,
  uploadProgress,
  resumedChunks,
  uploadFile,
  resetUploadProgress,
} = useFileUpload();

onMounted(() => {
  history.value = JSON.parse(localStorage.getItem("clipHistory") || "[]") as ClipRecord[];
});

watch(
  () => route.query.tab,
  (tab) => {
    if (tab === "file") activeTab.value = "file";
  },
);

const formatSize = (bytes: number) => {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + (sizes[i] || "B");
};

const handleFileSelect = (event: Event) => {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] || null;
  resetUploadProgress();
};

const requestJSON = async <T extends Record<string, unknown> = Record<string, unknown>>(
  url: string,
  options?: RequestInit,
): Promise<T> => {
  const response = await fetch(backendURL(url), options);
  const data = (await response.json().catch(() => ({}))) as T;
  if (!response.ok) {
    if (response.status === 404) throw new Error(t("home.notFound"));
    if (response.status === 413) throw new Error(t("clip.fileTooLarge"));
    if (response.status === 400 || response.status === 422)
      throw new Error(t("clip.invalidRequest"));
    throw new Error(t("common.requestFailed", { status: response.status }));
  }
  return data;
};

const submitClip = async () => {
  loading.value = true;
  errorMsg.value = "";

  try {
    const count = Number(formData.value.count);
    const expire =
      Number(formData.value.expire) * parseInt(formData.value.expireUnit);
    let data: { code: string };

    if (activeTab.value === "file") {
      if (!selectedFile.value) {
        errorMsg.value = t("clip.selectFileRequired");
        loading.value = false;
        return;
      }
      data = await uploadFile(selectedFile.value, { count, expire });
    } else {
      const fd = new FormData();
      fd.append("count", String(count));
      fd.append("expire", String(expire));
      fd.append("content", formData.value.content);
      fd.append("link", activeTab.value === "link" ? "yes" : "no");
      data = await requestJSON<{ code: string }>("/api/clip/create", { method: "POST", body: fd });
    }

    if (typeof data.code === "string") {
      const filename = selectedFile.value?.name || "";
      const size = selectedFile.value?.size || 0;
      const expiresAt = new Date(Date.now() + expire * 1000).toISOString();
      addToHistory(
        data.code,
        activeTab.value,
        filename,
        size,
        expiresAt,
        count,
      );
      detailItem.value = {
        code: data.code,
        type: activeTab.value as Exclude<CreateTab, "history">,
        filename,
        size,
        expiresAt,
        remainingCount: count,
        maxCount: count,
      };
      detailModalOpen.value = true;

      // Handle redirect
      const redirectUrl = route.query.redirect;
      if (redirectUrl) {
        const url = new URL(String(redirectUrl));
        url.searchParams.set("filecode", data.code);
        pendingRedirect.value = url.toString();
      }

      formData.value.content = "";
      selectedFile.value = null;
      resetUploadProgress();
    } else {
      errorMsg.value = t("clip.createFailed");
      toast.error(errorMsg.value);
    }
  } catch (err: unknown) {
    errorMsg.value =
      err instanceof TypeError
        ? t("common.networkError")
        : err instanceof Error
          ? err.message
          : t("common.unexpectedError");
    toast.error(errorMsg.value);
  } finally {
    loading.value = false;
  }
};

const getDomain = (url: unknown) => {
  try {
    return new URL(String(url)).hostname;
  } catch {
    return String(url);
  }
};

const doRedirect = () => {
  if (pendingRedirect.value) {
    window.location.href = pendingRedirect.value;
  }
};

const addToHistory = (
  code: string,
  type: CreateTab,
  filename = "",
  size = 0,
  expiresAt = "",
  maxCount = 0,
) => {
  if (type === "history") return;
  const entry: ClipRecord = {
    code,
    type,
    filename,
    size,
    expiresAt,
    remainingCount: maxCount,
    maxCount,
  };
  history.value.unshift(entry);
  if (history.value.length > 10) history.value.pop();
  localStorage.setItem("clipHistory", JSON.stringify(history.value));
};

const clearHistory = () => {
  history.value = [];
  localStorage.removeItem("clipHistory");
  toast.success(t("clip.historyCleared"));
};

const openHistoryItem = async (item: ClipRecord) => {
  try {
    const info = await requestJSON<{
      type: ClipKind;
      filename?: string;
      size?: number;
      expires_at: string;
      remaining_count: number;
      max_count: number;
      expired: boolean;
    }>(`/api/clip/${item.code}/info`);
    const refreshed: ClipRecord = {
      ...item,
      type: info.type === "text/plain" ? "text" : info.type,
      filename: info.filename || item.filename || "",
      size: info.size ?? item.size ?? 0,
      expiresAt: info.expires_at,
      remainingCount: info.remaining_count,
      maxCount: info.max_count,
      expired: info.expired,
    };
    detailItem.value = refreshed;
    const index = history.value.findIndex((entry) => entry.code === item.code);
    if (index !== -1) history.value[index] = refreshed;
    localStorage.setItem("clipHistory", JSON.stringify(history.value));
    detailModalOpen.value = true;
  } catch (error: unknown) {
    toast.error(
      error instanceof TypeError
        ? t("common.networkError")
        : error instanceof Error
          ? error.message
          : t("common.unexpectedError"),
    );
  }
};

const getIconComponent = (type: ClipKind): Component => {
  if (type === "file") return FileIcon;
  if (type === "link") return LinkIcon;
  return FileTextIcon;
};
</script>
