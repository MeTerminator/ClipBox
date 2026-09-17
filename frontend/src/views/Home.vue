<template>
  <div class="mx-auto flex min-h-[calc(100vh-15rem)] max-w-xl items-center">
    <Card class="w-full">
      <CardHeader class="text-center">
        <CardTitle class="text-2xl">{{ t("home.pickupTitle") }}</CardTitle>
        <CardDescription>{{ t("home.pickupSubtitle") }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="space-y-6" @submit.prevent="pickup">
          <div
            class="grid grid-cols-5 gap-2 sm:gap-3"
            role="group"
            :aria-label="t('home.pickupCode')"
          >
            <Input
              v-for="(_, index) in digits"
              :key="index"
              :ref="(element) => setInputRef(element, index)"
              v-model="digits[index]"
              class="h-14 text-center font-mono text-xl font-semibold sm:h-16 sm:text-2xl"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="1"
              pattern="[0-9]"
              :aria-label="t('home.digitLabel', { number: index + 1 })"
              @input="handleInput(index, $event)"
              @keydown="handleKeydown(index, $event)"
              @paste="handlePaste"
            />
          </div>

          <Button type="submit" class="w-full" size="lg" :disabled="loading">
            <CloudDownloadIcon />
            {{ loading ? t("home.loading") : t("home.pickupBtn") }}
          </Button>
        </form>
      </CardContent>
      <CardFooter class="grid grid-cols-1 gap-2 sm:grid-cols-3">
        <Button as-child variant="outline">
          <router-link :to="{ name: 'create', query: { tab: 'file' } }">
            <UploadCloudIcon />
            {{ t("home.sendFile") }}
          </router-link>
        </Button>
        <Button variant="outline" @click="historyOpen = true">
          <HistoryIcon />
          {{ t("home.pickupHistory") }}
        </Button>
        <Button as-child variant="outline">
          <router-link :to="{ name: 'rooms' }">
            <MessagesSquareIcon />
            {{ t("rooms.title") }}
          </router-link>
        </Button>
      </CardFooter>
    </Card>
  </div>

  <PickupResultModal
    :open="resultOpen"
    :result="pickupResult"
    @close="resultOpen = false"
    @copied="toast.success(t('clip.copied'))"
  />
  <PickupHistoryModal
    :open="historyOpen"
    :records="pickupHistory"
    @close="historyOpen = false"
    @select="openHistoryRecord"
    @clear="clearPickupHistory"
  />
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, type ComponentPublicInstance } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import {
  CloudDownloadIcon,
  HistoryIcon,
  MessagesSquareIcon,
  UploadCloudIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import PickupHistoryModal from "@/components/PickupHistoryModal.vue";
import PickupResultModal from "@/components/PickupResultModal.vue";
import type { ClipRecord } from "@/types";
import { backendURL } from "@/lib/backend";

const { t } = useI18n();
const digits = ref<string[]>(["", "", "", "", ""]);
const inputRefs = ref<HTMLInputElement[]>([]);
const loading = ref(false);
const pickupResult = ref<ClipRecord | null>(null);
const resultOpen = ref(false);
const historyOpen = ref(false);
const pickupHistory = ref<ClipRecord[]>(loadHistory());

function loadHistory(): ClipRecord[] {
  try {
    return JSON.parse(localStorage.getItem("pickupHistory") || "[]") as ClipRecord[];
  } catch {
    return [];
  }
}

const setInputRef = (
  element: Element | ComponentPublicInstance | null,
  index: number,
) => {
  const input = element instanceof HTMLInputElement
    ? element
    : element && "$el" in element
      ? (element.$el as HTMLInputElement)
      : undefined;
  if (input) inputRefs.value[index] = input;
};

const handleInput = (index: number, event: Event) => {
  const target = event.target as HTMLInputElement;
  const value = target.value.replace(/\D/g, "").slice(-1);
  digits.value[index] = value;
  if (value && index < digits.value.length - 1) {
    nextTick(() => inputRefs.value[index + 1]?.focus());
  } else if (value && index === digits.value.length - 1) {
    nextTick(pickup);
  }
};

const handleKeydown = (index: number, event: KeyboardEvent) => {
  if (event.key === "Backspace" && !digits.value[index] && index > 0) {
    digits.value[index - 1] = "";
    nextTick(() => inputRefs.value[index - 1]?.focus());
  }
};

const handlePaste = (event: ClipboardEvent) => {
  event.preventDefault();
  const pasted = event.clipboardData?.getData("text").replace(/\D/g, "").slice(0, 5) || "";
  pasted.split("").forEach((digit, index) => {
    digits.value[index] = digit;
  });
  nextTick(() => inputRefs.value[Math.min(pasted.length, 4)]?.focus());
  if (pasted.length === 5) nextTick(pickup);
};

const handlePageDigit = (event: KeyboardEvent) => {
  if (event.metaKey || event.ctrlKey || event.altKey || resultOpen.value || historyOpen.value || !/^\d$/.test(event.key)) return;
  const target = event.target;
  if (target instanceof HTMLElement && target.matches("input, textarea, select, [contenteditable='true']")) return;

  const index = digits.value.findIndex((digit) => !digit);
  if (index === -1) return;

  event.preventDefault();
  digits.value[index] = event.key;
  if (index < digits.value.length - 1) nextTick(() => inputRefs.value[index + 1]?.focus());
  else nextTick(pickup);
};

const pickup = async () => {
  const code = digits.value.join("");
  if (!/^\d{5}$/.test(code)) {
    toast.error(t("home.invalidCode"));
    return;
  }
  if (loading.value) return;
  loading.value = true;
  try {
    const response = await fetch(backendURL(`/clip/${code}/resolve`), {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    const data = (await response.json().catch(() => ({}))) as ClipRecord;
    if (!response.ok) {
      throw new Error(
        response.status === 404
          ? t("home.notFound")
          : t("common.requestFailed", { status: response.status }),
      );
    }
    pickupResult.value = data;
    const record = { ...data, retrievedAt: new Date().toISOString() };
    pickupHistory.value = [
      record,
      ...pickupHistory.value.filter((item) => item.code !== data.code),
    ].slice(0, 10);
    localStorage.setItem("pickupHistory", JSON.stringify(pickupHistory.value));
    resultOpen.value = true;
  } catch (error) {
    toast.error(
      error instanceof TypeError
        ? t("common.networkError")
        : error instanceof Error
          ? error.message
          : t("common.unexpectedError"),
    );
  } finally {
    loading.value = false;
  }
};

const openHistoryRecord = (record: ClipRecord) => {
  historyOpen.value = false;
  pickupResult.value = record;
  resultOpen.value = true;
};

const clearPickupHistory = () => {
  pickupHistory.value = [];
  localStorage.removeItem("pickupHistory");
  historyOpen.value = false;
  toast.success(t("home.pickupHistoryCleared"));
};

onMounted(() => window.addEventListener("keydown", handlePageDigit));
onBeforeUnmount(() => window.removeEventListener("keydown", handlePageDigit));
</script>
