<template>
  <Card
    class="border border-border bg-card shadow-none p-6 animate-in fade-in duration-200"
  >
    <!-- Header Back Link -->
    <div class="mb-6">
      <router-link
        :to="{ name: 'home' }"
        class="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeftIcon class="h-4 w-4" />
        {{ $t("clip.backToHome") }}
      </router-link>
    </div>

    <!-- Redirect notice banner -->
    <div
      v-if="route.query.redirect"
      class="flex items-start gap-2.5 p-3 border border-border bg-transparent text-foreground rounded-lg text-sm mb-6"
    >
      <InfoIcon class="h-4 w-4 mt-0.5 shrink-0" />
      <span>{{
        $t("clip.redirectNotice", { domain: getDomain(route.query.redirect) })
      }}</span>
    </div>

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
          <Textarea
            v-model="formData.content"
            class="min-h-[150px] bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground"
            required
          />
        </div>

        <!-- Link Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'link'">
          <Label class="text-sm font-semibold text-muted-foreground">{{
            $t("clip.linkUrl")
          }}</Label>
          <Input
            v-model="formData.content"
            type="url"
            class="bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground"
            required
          />
        </div>

        <!-- File Tab Input -->
        <div class="flex flex-col gap-2" v-if="activeTab === 'file'">
          <Label class="text-sm font-semibold text-muted-foreground">{{
            $t("clip.selectFile")
          }}</Label>
          <div
            class="rounded-lg p-8 text-center cursor-pointer transition-colors duration-200 flex flex-col items-center justify-center min-h-[150px] border-border hover:border-foreground border-dashed border"
            @click="fileInput.click()"
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
            <div class="h-2 overflow-hidden rounded-full bg-muted">
              <div
                class="h-full bg-foreground transition-[width] duration-200"
                :style="{ width: `${uploadProgress}%` }"
              />
            </div>
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
            <Input
              v-model="formData.count"
              type="number"
              min="1"
              class="bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground"
            />
          </div>
          <div class="flex flex-col gap-2">
            <Label class="text-sm font-semibold text-muted-foreground">{{
              $t("clip.expireLabel")
            }}</Label>
            <div class="flex gap-2">
              <Input
                v-model="formData.expire"
                type="number"
                min="1"
                class="flex-1 bg-transparent border-border focus-visible:ring-1 focus-visible:ring-foreground"
              />
              <Select v-model="formData.expireUnit">
                <SelectTrigger
                  class="w-[110px] bg-transparent border-border focus:ring-1 focus:ring-foreground"
                >
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
          class="w-full font-semibold shadow-none border border-foreground bg-foreground text-background hover:bg-background hover:text-foreground py-6 transition-colors"
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
      <div
        v-if="pendingRedirect"
        class="mt-6 p-4 border border-border bg-transparent rounded-lg space-y-3"
      >
        <p class="text-muted-foreground text-sm">
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
            class="text-xs border-border bg-transparent hover:bg-foreground hover:text-background"
          >
            {{ $t("clip.stayBtn") }}
          </Button>
        </div>
      </div>

      <!-- Error Message Box -->
      <div
        v-if="errorMsg"
        class="mt-6 p-4 border border-destructive bg-transparent rounded-lg text-sm text-destructive"
      >
        {{ $t("clip.error") }}{{ errorMsg }}
      </div>
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
          class="flex items-center justify-between p-3 rounded-lg border border-border bg-transparent hover:border-foreground transition-colors"
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

<script setup>
import { ref, onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { Card } from "@/components/ui/card";
import FileDetailModal from "@/components/FileDetailModal.vue";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "vue-sonner";
import { useFileUpload } from "@/composables/useFileUpload";
import {
  ArrowLeftIcon,
  UploadCloudIcon,
  Trash2Icon,
  FileTextIcon,
  LinkIcon,
  FileIcon,
  ExternalLinkIcon,
  InfoIcon,
} from "lucide-vue-next";

const route = useRoute();
const { t } = useI18n();

const activeTab = ref(route.query.tab === "file" ? "file" : "text");
const formData = ref({
  content: "",
  count: 1000,
  expire: 1,
  expireUnit: "86400",
});
const selectedFile = ref(null);
const loading = ref(false);
const errorMsg = ref("");
const pendingRedirect = ref(null);
const history = ref([]);
const detailModalOpen = ref(false);
const detailItem = ref(null);
const fileInput = ref(null);
const {
  uploadStage,
  uploadProgress,
  resumedChunks,
  uploadFile,
  resetUploadProgress,
} = useFileUpload();

onMounted(() => {
  history.value = JSON.parse(localStorage.getItem("clipHistory") || "[]");
});

watch(
  () => route.query.tab,
  (tab) => {
    if (tab === "file") activeTab.value = "file";
  },
);

const formatSize = (bytes) => {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
};

const handleFileSelect = (e) => {
  selectedFile.value = e.target.files[0];
  resetUploadProgress();
};

const requestJSON = async (url, options) => {
  const response = await fetch(url, options);
  const data = await response.json().catch(() => ({}));
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
    let data;

    if (activeTab.value === "file") {
      if (!selectedFile.value) {
        errorMsg.value = t("clip.selectFileRequired");
        loading.value = false;
        return;
      }
      data = await uploadFile(selectedFile.value, { count, expire });
    } else {
      const fd = new FormData();
      fd.append("count", count);
      fd.append("expire", expire);
      fd.append("content", formData.value.content);
      fd.append("link", activeTab.value === "link" ? "yes" : "no");
      data = await requestJSON("/clip/create", { method: "POST", body: fd });
    }

    if (data.code) {
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
        type: activeTab.value,
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
  } catch (err) {
    errorMsg.value =
      err instanceof TypeError
        ? t("common.networkError")
        : err.message || t("common.unexpectedError");
    toast.error(errorMsg.value);
  } finally {
    loading.value = false;
  }
};

const getDomain = (url) => {
  try {
    return new URL(String(url)).hostname;
  } catch (e) {
    return String(url);
  }
};

const doRedirect = () => {
  if (pendingRedirect.value) {
    window.location.href = pendingRedirect.value;
  }
};

const addToHistory = (
  code,
  type,
  filename = "",
  size = 0,
  expiresAt = "",
  maxCount = 0,
) => {
  const entry = {
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

const openHistoryItem = async (item) => {
  try {
    const info = await requestJSON(`/clip/${item.code}/info`);
    const refreshed = {
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
  } catch (error) {
    toast.error(
      error instanceof TypeError
        ? t("common.networkError")
        : error.message || t("common.unexpectedError"),
    );
  }
};

const getIconComponent = (type) => {
  if (type === "file") return FileIcon;
  if (type === "link") return LinkIcon;
  return FileTextIcon;
};
</script>
