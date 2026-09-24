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
              class="aspect-square h-auto min-w-0 p-0 text-center font-mono !text-5xl font-bold sm:!text-6xl"
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
        <Button variant="outline" @click="openHistory">
          <HistoryIcon />
          {{ t("home.pickupHistory") }}
        </Button>
        <Button variant="outline" :disabled="creatingRoom" @click="createShareRoom">
          <MessagesSquareIcon />
          {{ creatingRoom ? t("home.creatingRoom") : t("home.createShareRoom") }}
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
    :records="historyRecords"
    @close="historyOpen = false"
    @select="openHistoryRecord"
    @clear="clearPickupHistory"
  />

  <Dialog :open="roomPasswordOpen" @update:open="(value) => !value && (roomPasswordOpen = value)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t("rooms.password") }}</DialogTitle>
        <DialogDescription>{{ t("home.roomPasswordDescription") }}</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="submitRoomPassword">
        <Input v-model="roomPasswordDraft" type="password" autocomplete="current-password" autofocus />
        <DialogFooter>
          <Button type="button" variant="outline" @click="roomPasswordOpen = false">{{ t("clip.stayBtn") }}</Button>
          <Button type="submit" :disabled="loading">{{ t("rooms.join") }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, type ComponentPublicInstance } from "vue";
import { useRouter } from "vue-router";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import PickupHistoryModal from "@/components/PickupHistoryModal.vue";
import PickupResultModal from "@/components/PickupResultModal.vue";
import type { ClipRecord, PickupHistoryRecord, RoomHistoryRecord, RoomSession, SavedRoom } from "@/types";
import { backendURL } from "@/lib/backend";

const { t } = useI18n();
const router = useRouter();
const digits = ref<string[]>(["", "", "", "", ""]);
const inputRefs = ref<HTMLInputElement[]>([]);
const loading = ref(false);
const pickupResult = ref<ClipRecord | null>(null);
const resultOpen = ref(false);
const historyOpen = ref(false);
const pickupHistory = ref<ClipRecord[]>(loadHistory());
const savedRooms = ref<SavedRoom[]>(loadSavedRooms());
const creatingRoom = ref(false);
const roomPasswordOpen = ref(false);
const roomPasswordDraft = ref("");
const pendingRoomCode = ref("");
const httpUnauthorized = 401;

const roomHistory = computed<RoomHistoryRecord[]>(() => savedRooms.value.map((room) => ({
  kind: "room",
  id: room.id,
  name: room.name,
  nickname: room.nickname,
  token: room.token,
  isOwner: Boolean(room.isOwner),
  hasPassword: Boolean(room.hasPassword),
  joinedAt: room.joinedAt || new Date(0).toISOString(),
})));
const historyRecords = computed<PickupHistoryRecord[]>(() => {
  const records: PickupHistoryRecord[] = [...roomHistory.value, ...pickupHistory.value];
  return records.sort((left, right) => historyTime(right) - historyTime(left));
});

function historyTime(record: PickupHistoryRecord): number {
  const value = record.kind === "room" ? record.joinedAt : record.retrievedAt;
  const time = value ? Date.parse(value) : Number.NaN;
  return Number.isNaN(time) ? 0 : time;
}

function loadSavedRooms(): SavedRoom[] {
  try {
    return JSON.parse(localStorage.getItem("shareRooms") || "[]") as SavedRoom[];
  } catch {
    return [];
  }
}

function saveRoomSession(value: RoomSession) {
  const member = value.room.members.find((item) => item.id === value.room.current_member_id);
  const entry: SavedRoom = {
    id: value.room.id,
    name: value.room.name,
    nickname: member?.nickname || "",
    token: value.token,
    isOwner: Boolean(member?.is_owner),
    hasPassword: Boolean(value.room.has_password),
    joinedAt: new Date().toISOString(),
  };
  savedRooms.value = [entry, ...savedRooms.value.filter((room) => room.id !== entry.id)].slice(0, 20);
  localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
}

function deviceInfo() {
  const ua = navigator.userAgent;
  let os = "Unknown OS";
  let browser = "Unknown browser";
  let device = /Mobile|Android|iPhone|iPad/i.test(ua) ? "Mobile" : "Desktop";
  if (/iPhone/.test(ua)) { device = "iPhone"; os = "iOS"; }
  else if (/iPad/.test(ua)) { device = "iPad"; os = "iPadOS"; }
  else if (/Android/.test(ua)) os = "Android";
  else if (/Windows/.test(ua)) os = "Windows";
  else if (/Mac OS X/.test(ua)) os = "macOS";
  else if (/Linux/.test(ua)) os = "Linux";
  if (/Edg\//.test(ua)) browser = "Edge";
  else if (/Firefox\//.test(ua)) browser = "Firefox";
  else if (/Chrome\//.test(ua)) browser = "Chrome";
  else if (/Safari\//.test(ua)) browser = "Safari";
  return { device, os, browser };
}

async function createShareRoom() {
  if (creatingRoom.value) return;
  creatingRoom.value = true;
  try {
    const response = await fetch(backendURL("/api/rooms"), {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ name: "", nickname: "", ...deviceInfo() }),
    });
    const data = (await response.json().catch(() => ({}))) as RoomSession;
    if (!response.ok || !data?.room?.id) throw new Error(t("rooms.unknownError"));
    saveRoomSession(data);
    await router.push({ name: "rooms", params: { roomID: data.room.id } });
  } catch (error) {
    toast.error(error instanceof TypeError ? t("common.networkError") : error instanceof Error ? error.message : t("rooms.unknownError"));
  } finally {
    creatingRoom.value = false;
  }
}

async function joinRoomByCode(code: string, password = "") {
  const response = await fetch(backendURL(`/api/rooms/${code}/join`), {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ nickname: "", password, ...deviceInfo() }),
  });
  const data = (await response.json().catch(() => ({}))) as RoomSession;
  if (response.status === httpUnauthorized) {
    pendingRoomCode.value = code;
    roomPasswordDraft.value = "";
    roomPasswordOpen.value = true;
    return false;
  }
  if (!response.ok || !data?.room?.id) throw new Error(t("rooms.unknownError"));
  saveRoomSession(data);
  await router.push({ name: "rooms", params: { roomID: data.room.id } });
  return true;
}

async function submitRoomPassword() {
  if (!pendingRoomCode.value || loading.value) return;
  loading.value = true;
  try {
    if (await joinRoomByCode(pendingRoomCode.value, roomPasswordDraft.value)) {
      roomPasswordOpen.value = false;
      pendingRoomCode.value = "";
    } else {
      toast.error(t("rooms.incorrectPassword"));
    }
  } catch (error) {
    toast.error(error instanceof TypeError ? t("common.networkError") : error instanceof Error ? error.message : t("rooms.unknownError"));
  } finally {
    loading.value = false;
  }
}

async function openHistory() {
  historyOpen.value = true;
  await refreshSavedRooms();
}

async function refreshSavedRooms() {
  const refreshed = await Promise.all(savedRooms.value.map(async (room) => {
    try {
      const response = await fetch(backendURL(`/api/rooms/${room.id}`), {
        headers: { Authorization: `Bearer ${room.token}`, Accept: "application/json" },
      });
      if (!response.ok) return null;
      const info = await response.json() as RoomSession["room"];
      const member = info.members.find((item) => item.id === info.current_member_id);
      return { ...room, name: info.name, nickname: member?.nickname || room.nickname, isOwner: Boolean(member?.is_owner), hasPassword: Boolean(info.has_password) };
    } catch {
      return room;
    }
  }));
  savedRooms.value = refreshed.filter((room): room is SavedRoom => room !== null);
  localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
}

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

const handlePagePaste = (event: ClipboardEvent) => {
  if (resultOpen.value || historyOpen.value || roomPasswordOpen.value) return;
  const target = event.target;
  if (target instanceof HTMLElement && target.matches("input, textarea, select, [contenteditable='true']")) return;

  const pasted = event.clipboardData?.getData("text").replace(/\D/g, "") || "";
  if (!/^\d{5}$/.test(pasted)) return;

  event.preventDefault();
  pasted.split("").forEach((digit, index) => { digits.value[index] = digit; });
  nextTick(pickup);
};

const handlePageDigit = (event: KeyboardEvent) => {
  if (event.metaKey || event.ctrlKey || event.altKey || resultOpen.value || historyOpen.value || roomPasswordOpen.value || !/^\d$/.test(event.key)) return;
  const target = event.target;
  if (target instanceof HTMLElement && target.matches("input, textarea, select, [contenteditable='true']")) return;

  const index = digits.value.findIndex((digit) => !digit);
  if (index === -1) return;

  event.preventDefault();
  digits.value[index] = event.key;
  if (index < digits.value.length - 1) nextTick(() => inputRefs.value[index + 1]?.focus());
  else nextTick(pickup);
};

async function roomExists(code: string) {
  try {
    const response = await fetch(backendURL(`/api/rooms/${code}`), { headers: { Accept: "application/json" } });
    return response.status === 200 || response.status === 401;
  } catch {
    return false;
  }
}

const pickup = async () => {
  const code = digits.value.join("");
  if (!/^\d{5}$/.test(code)) {
    toast.error(t("home.invalidCode"));
    return;
  }
  if (loading.value) return;
  loading.value = true;
  try {
    const response = await fetch(backendURL(`/api/clip/${code}/resolve`), {
      method: "POST",
      headers: { Accept: "application/json" },
    });
    const data = (await response.json().catch(() => ({}))) as ClipRecord;
    if (!response.ok) {
      if (response.status === 404 && await roomExists(code)) {
        const saved = savedRooms.value.find((room) => room.id === code);
        if (saved) {
          await router.push({ name: "rooms", params: { roomID: code } });
          return;
        }
        await joinRoomByCode(code);
        return;
      }
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

const openHistoryRecord = (record: PickupHistoryRecord) => {
  historyOpen.value = false;
  if (record.kind === "room") {
    void router.push({ name: "rooms", params: { roomID: record.id } });
    return;
  }
  pickupResult.value = record;
  resultOpen.value = true;
};

const clearPickupHistory = () => {
  pickupHistory.value = [];
  savedRooms.value = [];
  localStorage.removeItem("pickupHistory");
  localStorage.removeItem("shareRooms");
  historyOpen.value = false;
  toast.success(t("home.pickupHistoryCleared"));
};

onMounted(() => {
  window.addEventListener("keydown", handlePageDigit);
  window.addEventListener("paste", handlePagePaste);
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", handlePageDigit);
  window.removeEventListener("paste", handlePagePaste);
});
</script>
