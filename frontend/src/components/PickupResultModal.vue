<template>
  <Dialog :open="open" @update:open="(value) => !value && close()">
    <DialogContent v-if="result" class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <LinkIcon v-if="result.type === 'link'" />
          <FileIcon v-else-if="result.type === 'file'" />
          <FileTextIcon v-else />
          {{ resultTitle }}
        </DialogTitle>
        <DialogDescription>{{ result.code }}</DialogDescription>
      </DialogHeader>

      <div class="grid gap-3 sm:grid-cols-2">
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>{{ t("clip.expiresAt") }}</CardDescription>
            <CardTitle class="text-sm">{{ formatDate(result.expires_at) }}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader class="pb-2">
            <CardDescription>{{ t("home.remainingCount") }}</CardDescription>
            <CardTitle class="text-sm">{{ result.remaining_count }}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <template v-if="result.type === 'link'">
        <div class="rounded-lg bg-muted p-4 font-mono text-sm break-all">{{ result.content }}</div>
        <DialogFooter>
          <Button variant="outline" @click="close">{{ t("clip.stayBtn") }}</Button>
          <Button @click="openLink">{{ t("home.openLink") }}</Button>
        </DialogFooter>
      </template>

      <template v-else-if="result.type === 'text/plain' || result.type === 'text'">
        <ScrollArea class="max-h-80 rounded-lg border bg-muted/40 p-4">
          <pre class="whitespace-pre-wrap break-words font-mono text-sm">{{ result.content }}</pre>
        </ScrollArea>
        <Button class="w-full" @click="copyText">
          <CopyIcon />
          {{ t("home.copyText") }}
        </Button>
      </template>

      <template v-else>
        <div class="flex items-center gap-3 rounded-lg border p-4">
          <div class="rounded-md bg-muted p-3"><FileIcon /></div>
          <div class="min-w-0">
            <p class="truncate font-medium">{{ result.filename }}</p>
            <p class="text-sm text-muted-foreground">{{ formatSize(result.size) }}</p>
          </div>
        </div>
        <Button as-child class="w-full">
          <a :href="result.download_url" download>
            <DownloadIcon />
            {{ t("home.downloadFile") }}
          </a>
        </Button>
      </template>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { CopyIcon, DownloadIcon, FileIcon, FileTextIcon, LinkIcon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { ClipRecord } from "@/types";

const props = withDefaults(defineProps<{ open?: boolean; result?: ClipRecord | null }>(), {
  open: false,
  result: null,
});
const emit = defineEmits<{ close: []; copied: [] }>();
const { t, locale } = useI18n();

const resultTitle = computed(() => {
  if (props.result?.type === "link") return t("home.linkResult");
  if (props.result?.type === "file") return t("home.fileResult");
  return t("home.textResult");
});

const close = () => emit("close");
const openLink = () => window.location.assign(props.result?.content || "/");
const copyText = async () => {
  try {
    await navigator.clipboard.writeText(props.result?.content || "");
    emit("copied");
  } catch {
    // Keep the content visible if clipboard permission is denied.
  }
};
const formatDate = (value?: string) =>
  value
    ? new Intl.DateTimeFormat(locale.value === "zh" ? "zh-CN" : "en-US", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "-";
const formatSize = (bytes = 0) => {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index ? 2 : 0)} ${units[index]}`;
};
</script>
