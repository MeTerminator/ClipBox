<template>
  <Dialog :open="open" @update:open="(value) => !value && emit('close')">
    <DialogContent class="sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>{{ t("home.pickupHistory") }}</DialogTitle>
        <DialogDescription>{{ t("home.pickupHistorySubtitle") }}</DialogDescription>
      </DialogHeader>

      <ScrollArea class="max-h-[60vh] pr-3">
        <div v-if="records.length" class="space-y-2">
          <Button
            v-for="record in records"
            :key="record.kind === 'room' ? `room-${record.id}` : `clip-${record.code}-${record.retrievedAt}`"
            variant="outline"
            class="h-auto w-full justify-start px-4 py-3 text-left"
            @click="emit('select', record)"
          >
            <MessagesSquareIcon v-if="record.kind === 'room'" />
            <FileIcon v-else-if="record.type === 'file'" />
            <LinkIcon v-else-if="record.type === 'link'" />
            <FileTextIcon v-else />
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">
                {{ record.kind === "room" ? record.name || t("rooms.unnamed") : record.filename || typeLabel(record.type) }}
              </span>
              <span v-if="record.kind === 'room'" class="block text-xs text-muted-foreground">
                {{ record.id }} · {{ record.nickname }} · {{ formatDate(record.joinedAt) }}
              </span>
              <span v-else class="block text-xs text-muted-foreground">
                {{ record.code }} · {{ formatDate(record.retrievedAt) }}
              </span>
            </span>
            <Badge v-if="record.kind === 'room' && record.isOwner" variant="secondary">
              {{ t("rooms.owner") }}
            </Badge>
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
import { ChevronRightIcon, FileIcon, FileTextIcon, LinkIcon, MessagesSquareIcon } from "@lucide/vue";
import { Badge } from "@/components/ui/badge";
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
import type { ClipKind, PickupHistoryRecord } from "@/types";

withDefaults(defineProps<{ open?: boolean; records?: PickupHistoryRecord[] }>(), {
  open: false,
  records: () => [],
});

const emit = defineEmits<{
  close: [];
  select: [record: PickupHistoryRecord];
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
