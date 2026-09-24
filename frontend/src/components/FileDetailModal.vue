<template>
  <Dialog :open="open" @update:open="(value) => !value && close()">
    <DialogContent v-if="item" class="sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <FileIcon v-if="item.type === 'file'" />
          <LinkIcon v-else-if="item.type === 'link'" />
          <FileTextIcon v-else />
          {{ item.filename || typeLabel }}
        </DialogTitle>
        <DialogDescription class="flex flex-wrap gap-x-4 gap-y-1">
          <span v-if="item.expiresAt">{{ t("clip.expiresAt") }} {{ formatDate(item.expiresAt) }}</span>
          <span v-if="item.remainingCount !== undefined">{{ t("home.remainingCount") }} {{ item.remainingCount }}</span>
        </DialogDescription>
      </DialogHeader>

      <div class="grid gap-4 sm:grid-cols-2">
        <div class="space-y-4">
          <Card>
            <CardHeader>
              <CardDescription>{{ t("clip.pickupCode") }}</CardDescription>
              <CardTitle class="flex items-center justify-between font-mono text-3xl tracking-widest">
                {{ item.code }}
                <Button variant="ghost" size="icon" @click="copyCode"><CopyIcon /></Button>
              </CardTitle>
            </CardHeader>
          </Card>

          <div class="grid grid-cols-2 gap-2">
            <Button variant="outline" class="h-auto justify-start py-3" @click="copyWget">
              <TerminalIcon />
              <span class="text-left"><span class="block">wget</span><span class="block text-xs text-muted-foreground">{{ t("clip.copyWget") }}</span></span>
            </Button>
            <Button variant="outline" class="h-auto justify-start py-3" @click="copyCurl">
              <TerminalIcon />
              <span class="text-left"><span class="block">curl</span><span class="block text-xs text-muted-foreground">{{ t("clip.copyCurl") }}</span></span>
            </Button>
          </div>
        </div>

        <Card>
          <CardContent class="flex h-full min-h-64 flex-col items-center justify-center gap-3 pt-6">
            <div class="rounded-lg border bg-white p-3">
              <img v-if="qrDataUrl" :src="qrDataUrl" :alt="t('clip.qrAlt')" class="size-40" />
              <Loader2Icon v-else class="size-8 animate-spin text-muted-foreground" />
            </div>
            <p class="text-center text-sm text-muted-foreground">{{ t("clip.scanToPickup") }}</p>
          </CardContent>
        </Card>
      </div>

      <DialogFooter>
        <Button class="w-full" @click="copyLink">
          <CopyIcon />
          {{ copied ? t("clip.copied") : t("clip.copyPickupLink") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import QRCode from "qrcode";
import { useI18n } from "vue-i18n";
import { CopyIcon, FileIcon, FileTextIcon, LinkIcon, Loader2Icon, TerminalIcon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ClipRecord } from "@/types";
import { backendOrigin } from "@/lib/backend";

const props = withDefaults(defineProps<{ open?: boolean; item?: ClipRecord | null }>(), {
  open: false,
  item: null,
});
const emit = defineEmits<{ close: []; copied: [] }>();
const { t, locale } = useI18n();
const qrDataUrl = ref("");
const copied = ref(false);

const pickupLink = computed(() =>
  props.item?.code ? `${backendOrigin}/api/clip/${props.item.code}` : "",
);
const typeLabel = computed(() => t(`clip.${props.item?.type === "text/plain" ? "text" : props.item?.type || "file"}`));

watch(
  () => [props.open, props.item?.code] as const,
  async ([open]) => {
    copied.value = false;
    qrDataUrl.value = "";
    if (!open || !pickupLink.value) return;
    try {
      qrDataUrl.value = await QRCode.toDataURL(pickupLink.value, {
        width: 180,
        margin: 0,
        errorCorrectionLevel: "M",
      });
    } catch {
      qrDataUrl.value = "";
    }
  },
  { immediate: true },
);

const close = () => emit("close");
const formatDate = (value?: string) =>
  value
    ? new Intl.DateTimeFormat(locale.value === "zh" ? "zh-CN" : "en-US", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "";
const writeClipboard = async (value: string, markCopied = false) => {
  try {
    await navigator.clipboard.writeText(value);
    if (markCopied) copied.value = true;
    emit("copied");
  } catch {
    // Keep the dialog open if clipboard permission is denied.
  }
};
const copyCode = () => writeClipboard(props.item?.code || "");
const copyLink = () => writeClipboard(pickupLink.value, true);
const copyWget = () => writeClipboard(`wget -O "${props.item?.filename || "download"}" "${pickupLink.value}"`);
const copyCurl = () => writeClipboard(`curl -L -o "${props.item?.filename || "download"}" "${pickupLink.value}"`);
</script>
