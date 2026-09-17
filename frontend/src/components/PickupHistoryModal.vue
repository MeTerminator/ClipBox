<template>
  <Dialog :open="open" @update:open="(value) => !value && emit('close')">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>{{ t("home.pickupHistory") }}</DialogTitle>
        <DialogDescription>{{ t("home.pickupSubtitle") }}</DialogDescription>
      </DialogHeader>

      <ScrollArea class="max-h-[60vh] pr-3">
        <div v-if="records.length" class="space-y-2">
          <Button
            v-for="record in records"
            :key="`${record.code}-${record.retrievedAt}`"
            variant="outline"
            class="h-auto w-full justify-start px-4 py-3 text-left"
            @click="emit('select', record)"
          >
            <FileIcon v-if="record.type === 'file'" />
            <LinkIcon v-else-if="record.type === 'link'" />
            <FileTextIcon v-else />
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">
                {{ record.filename || typeLabel(record.type) }}
              </span>
              <span class="block text-xs text-muted-foreground">
                {{ record.code }} · {{ formatDate(record.retrievedAt) }}
              </span>
            </span>
            <ChevronRightIcon class="text-muted-foreground" />
          </Button>
        </div>
        <div v-else class="rounded-lg border border-dashed p-10 text-center text-sm text-muted-foreground">
          {{ t("home.noPickupHistory") }}
        </div>
      </ScrollArea>

      <DialogFooter v-if="records.length">
        <Button variant="outline" @click="emit('clear')">
          {{ t("home.clearPickupHistory") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { ChevronRightIcon, FileIcon, FileTextIcon, LinkIcon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { ClipKind, ClipRecord } from "@/types";

withDefaults(defineProps<{ open?: boolean; records?: ClipRecord[] }>(), {
  open: false,
  records: () => [],
});

const emit = defineEmits<{
  close: [];
  select: [record: ClipRecord];
  clear: [];
}>();
const { t, locale } = useI18n();

const typeLabel = (type: ClipKind) => {
  if (type === "file") return t("home.fileResult");
  if (type === "link") return t("home.linkResult");
  return t("home.textResult");
};

const formatDate = (value?: string) =>
  value
    ? new Intl.DateTimeFormat(locale.value === "zh" ? "zh-CN" : "en-US", {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(new Date(value))
    : "-";
</script>
