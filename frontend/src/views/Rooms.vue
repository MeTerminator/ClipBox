<template>
  <div v-if="!session" class="mx-auto max-w-xl space-y-6">
    <Button as-child variant="ghost" class="-ml-3">
      <router-link :to="{ name: 'home' }"><ArrowLeftIcon />{{ t("rooms.back") }}</router-link>
    </Button>

    <div class="space-y-2 text-center">
      <div class="mx-auto flex size-12 items-center justify-center rounded-lg bg-muted"><MessagesSquareIcon /></div>
      <h1 class="text-2xl font-semibold">{{ t("rooms.title") }}</h1>
      <p class="text-muted-foreground">{{ t("rooms.subtitle") }}</p>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ showCreate ? t("rooms.createTitle") : t("rooms.joinTitle") }}</CardTitle>
        <CardDescription>{{ showCreate ? t("rooms.roomNameOptional") : t("rooms.roomID") }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form v-if="showCreate" class="space-y-4" @submit.prevent="createRoom">
          <div class="space-y-2"><Label for="room-name">{{ t("rooms.roomName") }}</Label><Input id="room-name" v-model="createForm.name" maxlength="80" :placeholder="t('rooms.roomNameOptional')" /></div>
          <Button class="w-full" :disabled="busy" type="submit"><PlusIcon />{{ t("rooms.create") }}</Button>
        </form>
        <form v-else class="space-y-4" @submit.prevent="joinRoom(joinForm.id)">
          <div class="space-y-2"><Label for="room-id">{{ t("rooms.roomID") }}</Label><Input id="room-id" v-model="joinForm.id" maxlength="5" inputmode="numeric" pattern="[0-9]{5}" required /></div>
          <div class="space-y-2"><Label for="join-password">{{ t("rooms.password") }}</Label><Input id="join-password" v-model="joinForm.password" type="password" autocomplete="current-password" :placeholder="t('rooms.passwordJoinHint')" /></div>
          <Button class="w-full" :disabled="busy" type="submit"><LogInIcon />{{ t("rooms.join") }}</Button>
        </form>
      </CardContent>
      <CardFooter>
        <Button type="button" variant="ghost" class="w-full" @click="showCreate = !showCreate">
          <LogInIcon v-if="showCreate" /><PlusIcon v-else />
          {{ showCreate ? t("rooms.switchToJoin") : t("rooms.create") }}
        </Button>
      </CardFooter>
    </Card>

    <Card v-if="savedRooms.length">
      <CardHeader><CardTitle class="text-base">{{ t("rooms.savedRooms") }}</CardTitle></CardHeader>
      <CardContent class="space-y-2">
        <Button v-for="room in savedRooms" :key="room.id" variant="outline" class="h-auto w-full justify-between py-3" @click="openSavedRoom(room)">
          <span class="min-w-0 flex-1 text-left"><span class="block truncate font-medium">{{ room.name || t("rooms.unnamed") }}</span><span class="block truncate text-xs text-muted-foreground">{{ room.id }} · {{ room.nickname }}</span></span>
          <span class="flex shrink-0 items-center gap-1 text-muted-foreground"><LockIcon v-if="room.hasPassword" class="size-4" /><CrownIcon v-if="room.isOwner" class="size-4" /><ChevronRightIcon /></span>
        </Button>
      </CardContent>
    </Card>
  </div>

  <div v-else class="space-y-4">
    <Card>
      <CardHeader class="gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="space-y-1">
          <form v-if="editingName" class="flex gap-2" @submit.prevent="saveRoomName">
            <Input v-model="roomNameDraft" maxlength="80" autofocus />
            <Button type="submit" size="icon" :disabled="busy"><CheckIcon /></Button>
          </form>
          <CardTitle v-else class="flex items-center gap-2">
            {{ session.room.name || t("rooms.unnamed") }}
            <Button variant="ghost" size="icon-sm" @click="startEditingName"><PencilIcon /></Button>
          </CardTitle>
          <Button variant="link" class="h-auto p-0 font-mono text-muted-foreground" @click="copyRoomID">{{ session.room.id }} <CopyIcon /></Button>
        </div>
        <div class="flex flex-wrap gap-2">
          <Badge :variant="connected ? 'default' : 'secondary'" class="h-9 px-3">{{ connected ? t("rooms.online") : t("rooms.offline") }}</Badge>
          <Button variant="outline" size="sm" @click="copyInvite"><LinkIcon />{{ t("rooms.copyLink") }}</Button>
          <Button variant="outline" size="sm" @click="openPasswordDialog"><KeyRoundIcon />{{ t("rooms.setPassword") }}</Button>
          <AlertDialog v-if="me?.is_owner">
            <AlertDialogTrigger as-child><Button variant="destructive" size="sm"><Trash2Icon />{{ t("rooms.delete") }}</Button></AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader><AlertDialogTitle>{{ t("rooms.delete") }}</AlertDialogTitle><AlertDialogDescription>{{ t("rooms.deleteConfirm") }}</AlertDialogDescription></AlertDialogHeader>
              <AlertDialogFooter><AlertDialogCancel>{{ t("clip.stayBtn") }}</AlertDialogCancel><AlertDialogAction @click="deleteRoom">{{ t("rooms.delete") }}</AlertDialogAction></AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
          <Button variant="ghost" size="sm" @click="leaveRoom()"><LogOutIcon />{{ t("rooms.leave") }}</Button>
        </div>
      </CardHeader>
    </Card>

    <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
      <Card class="overflow-hidden">
        <CardContent class="p-0">
          <ScrollArea class="h-[520px] p-4">
            <div v-if="!messages.length" class="flex h-[460px] flex-col items-center justify-center gap-2 text-center text-muted-foreground"><MessagesSquareIcon class="size-8" /><p>{{ t("rooms.empty") }}</p></div>
            <div v-else class="space-y-4">
              <article v-for="message in messages" :key="message.id" :class="['flex flex-col gap-1', message.sender.id === session.room.current_member_id ? 'items-end' : 'items-start']">
                <div class="flex items-center gap-2 text-xs text-muted-foreground"><ClipboardIcon v-if="message.source === 'clipboard'" class="size-4 text-amber-500" :title="t('rooms.clipboard')" /><span>{{ message.sender.nickname }}</span><span>{{ formatTime(message.created_at) }}</span></div>
                <div v-if="message.kind === 'text'" class="max-w-[85%] whitespace-pre-wrap rounded-lg bg-muted px-3 py-2 text-sm">{{ message.text }}</div>
                <Button v-else-if="message.file" variant="outline" class="h-auto max-w-[85%] justify-start py-3" @click="downloadFile(message)"><FileIcon /><span class="min-w-0 text-left"><span class="block truncate font-medium">{{ message.file.name }}</span><span class="block text-xs text-muted-foreground">{{ formatSize(message.file.size) }}</span></span><DownloadIcon /></Button>
              </article>
            </div>
          </ScrollArea>
          <Separator />
          <form class="p-3" @submit.prevent="sendText">
            <input ref="fileInput" type="file" hidden @change="sendFile" />
            <InputGroup class="h-10">
              <InputGroupAddon>
                <InputGroupButton size="icon-sm" :title="t('rooms.attach')" :disabled="busy || !connected" @click="fileInput?.click()"><PaperclipIcon /></InputGroupButton>
              </InputGroupAddon>
              <InputGroupTextarea v-model="draft" class="h-10 min-h-10 py-2 leading-6" :placeholder="t('rooms.sendPlaceholder')" @keydown.enter.exact.prevent="sendText" />
              <InputGroupAddon align="inline-end">
                <InputGroupButton size="icon-sm" :title="t('rooms.send')" :disabled="busy || !connected || !draft.trim()" @click="sendText"><SendIcon /></InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          </form>
          <Progress v-if="uploadStage" :model-value="uploadProgress" class="h-1 rounded-none" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle class="flex items-center justify-between text-base">{{ t("rooms.members") }}<Badge variant="secondary">{{ session.room.members.length }}</Badge></CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <div v-for="member in session.room.members" :key="member.id" class="flex flex-wrap items-center gap-x-3 gap-y-2">
            <div class="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted" :title="member.os" :aria-label="member.os">
              <component :is="systemIcon(member)" class="size-4" />
            </div>
            <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
              <form v-if="editingNickname && member.id === me?.id" class="flex min-w-[min(100%,14rem)] flex-1 flex-wrap gap-2" @submit.prevent="saveNickname">
                <Input v-model="nicknameDraft" class="min-w-[8rem] flex-1" maxlength="40" autofocus />
                <Button type="submit" size="icon" :disabled="busy"><CheckIcon /></Button>
              </form>
              <div v-else class="flex min-w-[min(100%,10rem)] flex-1 flex-wrap items-center gap-1">
                <p class="flex min-w-0 items-center gap-1 truncate text-sm font-medium">{{ member.nickname }} <CrownIcon v-if="member.is_owner" class="size-3 shrink-0" /></p>
                <Button v-if="member.id === me?.id" variant="ghost" size="icon-sm" @click="startEditingNickname"><PencilIcon /></Button>
              </div>
              <p class="w-full truncate text-xs text-muted-foreground">{{ t("rooms.deviceInfo", member) }}</p>
            </div>
            <span :class="['size-2 shrink-0 rounded-full', member.online ? 'bg-emerald-500' : 'bg-muted-foreground/40']" />
          </div>
        </CardContent>
      </Card>
    </div>
  </div>

  <Dialog :open="passwordOpen" @update:open="(value) => !value && (passwordOpen = value)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t("rooms.setPassword") }}</DialogTitle>
        <DialogDescription>{{ t("rooms.passwordHint") }}</DialogDescription>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="saveRoomPassword">
        <Input v-model="passwordDraft" type="password" autocomplete="new-password" :placeholder="t('rooms.passwordPlaceholder')" />
        <DialogFooter>
          <Button type="button" variant="outline" @click="passwordOpen = false">{{ t("clip.stayBtn") }}</Button>
          <Button type="submit" :disabled="busy">{{ t("common.save") }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useFileUpload } from "@/composables/useFileUpload";
import { setRoomFileDropHandler } from "@/composables/fileDropTarget";
import { backendOrigin, backendURL, backendWebSocketURL, uploadBackendURL } from "@/lib/backend";
import { isDesktopClient, listenForDesktopCopies, setDesktopSharedText } from "@/lib/desktop";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupTextarea } from "@/components/ui/input-group";
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import AppleLogo from "@/components/icons/AppleLogo.vue";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { RoomInfo, RoomMember, RoomMessage, RoomSession, SavedRoom } from "@/types";
import { ArrowLeftIcon, CheckIcon, ChevronRightIcon, ClipboardIcon, CopyIcon, CrownIcon, DownloadIcon, FileIcon, KeyRoundIcon, LinkIcon, LockIcon, LogInIcon, LogOutIcon, MessagesSquareIcon, MonitorIcon, PaperclipIcon, PencilIcon, PlusIcon, SendIcon, SmartphoneIcon, TabletIcon, TerminalIcon, Trash2Icon } from "@lucide/vue";

class HTTPError extends Error { constructor(message: string, public readonly status: number) { super(message); } }

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const createForm = reactive({ name: "" });
const joinForm = reactive({ id: String(route.params.roomID || ""), password: "" });
const session = ref<RoomSession | null>(null);
const messages = ref<RoomMessage[]>([]);
const draft = ref("");
const busy = ref(false);
const connected = ref(false);
const fileInput = ref<HTMLInputElement | null>(null);
const showCreate = ref(false);
const editingName = ref(false);
const roomNameDraft = ref("");
const editingNickname = ref(false);
const nicknameDraft = ref("");
const passwordOpen = ref(false);
const passwordDraft = ref("");
const { uploadStage, uploadProgress, uploadFile, resetUploadProgress } = useFileUpload();
const savedRooms = ref<SavedRoom[]>(loadSavedRooms());
let socket: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let pingTimer: ReturnType<typeof setInterval> | undefined;
let intentionalClose = false;
let stopDesktopCopyListener: (() => void) | undefined;
const me = computed(() => session.value?.room.members.find((member) => member.id === session.value?.room.current_member_id));

function errorMessage(error: unknown) { return error instanceof Error ? error.message : t("rooms.unknownError"); }
function loadSavedRooms(): SavedRoom[] { try { return JSON.parse(localStorage.getItem("shareRooms") || "[]") as SavedRoom[]; } catch { return []; } }
function saveSession(value: RoomSession) {
  const existing = savedRooms.value.find((room) => room.id === value.room.id);
  const member = value.room.members.find((item) => item.id === value.room.current_member_id);
  const entry: SavedRoom = {
    id: value.room.id,
    name: value.room.name,
    nickname: member?.nickname || existing?.nickname || "",
    token: value.token,
    isOwner: Boolean(member?.is_owner),
    hasPassword: Boolean(value.room.has_password),
    joinedAt: existing?.joinedAt || new Date().toISOString(),
  };
  savedRooms.value = [entry, ...savedRooms.value.filter((room) => room.id !== entry.id)].slice(0, 20);
  localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
}
function deviceInfo() {
  const ua = navigator.userAgent;
  let os = "Unknown OS"; let browser = "Unknown browser"; let device = /Mobile|Android|iPhone|iPad/i.test(ua) ? "Mobile" : "Desktop";
  if (/iPhone/.test(ua)) { device = "iPhone"; os = "iOS"; } else if (/iPad/.test(ua)) { device = "iPad"; os = "iPadOS"; } else if (/Android/.test(ua)) os = "Android"; else if (/Windows/.test(ua)) os = "Windows"; else if (/Mac OS X/.test(ua)) os = "macOS"; else if (/Linux/.test(ua)) os = "Linux";
  if (/Edg\//.test(ua)) browser = "Edge"; else if (/Firefox\//.test(ua)) browser = "Firefox"; else if (/Chrome\//.test(ua)) browser = "Chrome"; else if (/Safari\//.test(ua)) browser = "Safari";
  return { device, os, browser };
}
function systemIcon(member: Pick<RoomMember, "os" | "device">) {
  const identity = `${member.os} ${member.device}`.toLowerCase();
  if (identity.includes("macos") || identity.includes("ios")) return AppleLogo;
  if (identity.includes("ipad")) return TabletIcon;
  if (identity.includes("iphone") || identity.includes("android")) return SmartphoneIcon;
  if (identity.includes("linux")) return TerminalIcon;
  return MonitorIcon;
}

async function request<T>(url: string, options: RequestInit = {}, token = session.value?.token): Promise<T> {
  const headers = new Headers(options.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const response = await fetch(backendURL(url), { ...options, headers });
  if (!response.ok) { const body = await response.json().catch(() => ({})) as { error?: string }; throw new HTTPError(body.error || t("rooms.unknownError"), response.status); }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
async function createRoom() { busy.value = true; try { activate(await request<RoomSession>("/api/rooms", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ...createForm, nickname: "", ...deviceInfo() }) }, undefined)); } catch (error) { toast.error(errorMessage(error)); } finally { busy.value = false; } }
async function joinRoom(id: string) { busy.value = true; try { const normalized = id.trim().toUpperCase(); activate(await request<RoomSession>(`/api/rooms/${normalized}/join`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ nickname: "", password: joinForm.password, ...deviceInfo() }) }, undefined)); } catch (error) { toast.error(error instanceof HTTPError && error.status === 404 ? t("rooms.roomNotFound") : error instanceof HTTPError && error.status === 401 ? t("rooms.incorrectPassword") : errorMessage(error)); } finally { busy.value = false; } }
function activate(data: RoomSession) { closeSocket(); session.value = data; messages.value = []; saveSession(data); void router.replace({ name: "rooms", params: { roomID: data.room.id } }); connectSocket(); }
async function openSavedRoom(room: SavedRoom) { busy.value = true; try { activate({ room: await request<RoomInfo>(`/api/rooms/${room.id}`, {}, room.token), token: room.token }); } catch { savedRooms.value = savedRooms.value.filter((saved) => saved.id !== room.id); localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value)); toast.error(t("rooms.roomNotFound")); } finally { busy.value = false; } }
function connectSocket() {
  if (!session.value) return;
  connected.value = false; intentionalClose = false;
  const connection = socket = new WebSocket(backendWebSocketURL(`/api/rooms/${session.value.room.id}/ws`));
  connection.addEventListener("open", () => { connection.send(JSON.stringify({ type: "auth", token: session.value?.token })); clearInterval(pingTimer); pingTimer = setInterval(() => sendSocket({ type: "ping" }), 20000); });
  connection.addEventListener("message", (event) => void handleSocketEvent(event));
  connection.addEventListener("close", () => { if (connection !== socket) return; connected.value = false; clearInterval(pingTimer); if (!intentionalClose && session.value) { clearTimeout(reconnectTimer); reconnectTimer = setTimeout(connectSocket, 1500); } });
  connection.addEventListener("error", () => connection.close());
}
async function handleSocketEvent(event: MessageEvent<string>) {
  let data: Record<string, unknown>; try { data = JSON.parse(event.data) as Record<string, unknown>; } catch { return; }
  if (data.type === "ready") { connected.value = true; if (session.value) session.value.room = data.room as RoomInfo; messages.value = (data.messages as RoomMessage[]) || []; const latestClipboard = [...messages.value].reverse().find((message) => message.kind === "text" && message.source === "clipboard"); await setDesktopSharedText(latestClipboard?.text || null); await scrollBottom(); }
  else if (data.type === "message") { const message = data.message as RoomMessage; if (!messages.value.some((item) => item.id === message.id)) messages.value.push(message); if (message.kind === "text" && message.source === "clipboard") await setDesktopSharedText(message.text); await scrollBottom(); }
  else if (data.type === "presence" && session.value) { session.value.room.members = (data.members as RoomInfo["members"]) || []; saveSession(session.value); }
  else if (data.type === "member_updated" && session.value) {
    const member = data.member as RoomInfo["members"][number];
    session.value.room.members = session.value.room.members.map((item) => item.id === member.id ? member : item);
    saveSession(session.value);
  }
  else if (data.type === "room_updated" && session.value) {
    session.value.room.name = String(data.name || session.value.room.name);
    if ("has_password" in data) session.value.room.has_password = Boolean(data.has_password);
    saveSession(session.value);
  }
  else if (data.type === "room_deleted") { toast.error(t("rooms.roomNotFound")); leaveRoom(); }
  else if (data.type === "error") toast.error(t("rooms.unknownError"));
}
function sendSocket(command: object) { if (!socket || socket.readyState !== WebSocket.OPEN) throw new Error(t("rooms.unknownError")); socket.send(JSON.stringify(command)); }
function sendMessage(payload: object) { sendSocket({ type: "send", message: payload }); }
function sendText() { const text = draft.value.trim(); if (!text || busy.value) return; try { sendMessage({ kind: "text", source: "user", text }); draft.value = ""; } catch (error) { toast.error(errorMessage(error)); } }
async function processRoomFile(file: File) { if (busy.value) return; if (!connected.value) { toast.error(t("rooms.unknownError")); return; } busy.value = true; resetUploadProgress(); try { const result = await uploadFile(file, { count: 1000, expire: 31536000 }); sendMessage({ kind: "file", source: "user", file_code: result.code }); } catch (error) { toast.error(errorMessage(error)); } finally { busy.value = false; resetUploadProgress(); } }
async function sendFile(event: Event) { const input = event.target as HTMLInputElement; const file = input.files?.[0]; input.value = ""; if (file) await processRoomFile(file); }
async function downloadFile(message: RoomMessage) {
  if (!message.file || !session.value) return;
  try {
    const headers = { Authorization: `Bearer ${session.value.token}` };
    const directURL = await uploadBackendURL(message.file.download_url);
    let response: Response;
    try {
      response = await fetch(directURL, { headers });
    } catch {
      response = await fetch(backendURL(message.file.download_url), { headers });
    }
    if (!response.ok && directURL !== backendURL(message.file.download_url)) {
      response = await fetch(backendURL(message.file.download_url), { headers });
    }
    if (!response.ok) throw new Error(t("rooms.unknownError"));
    const url = URL.createObjectURL(await response.blob());
    const link = document.createElement("a");
    link.href = url;
    link.download = message.file.name;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (error) {
    toast.error(errorMessage(error));
  }
}
async function copyInvite() { if (!session.value) return; await navigator.clipboard.writeText(`${backendOrigin}/rooms/${session.value.room.id}`); toast.success(t("rooms.copied")); }
async function copyRoomID() { if (!session.value) return; await navigator.clipboard.writeText(session.value.room.id); toast.success(t("rooms.roomIDCopied")); }
async function deleteRoom() { if (!session.value) return; try { await request<void>(`/api/rooms/${session.value.room.id}`, { method: "DELETE" }); leaveRoom(); } catch (error) { toast.error(error instanceof HTTPError && error.status === 403 ? t("rooms.onlyOwner") : errorMessage(error)); } }
function startEditingName() { roomNameDraft.value = session.value?.room.name || ""; editingName.value = true; }
async function saveRoomName() { if (!session.value) return; busy.value = true; try { const result = await request<{ name: string }>(`/api/rooms/${session.value.room.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: roomNameDraft.value }) }); session.value.room.name = result.name; editingName.value = false; } catch (error) { toast.error(errorMessage(error)); } finally { busy.value = false; } }
function startEditingNickname() { nicknameDraft.value = me.value?.nickname || ""; editingNickname.value = true; }
async function saveNickname() {
  if (!session.value || !me.value) return;
  busy.value = true;
  try {
    const result = await request<{ nickname: string }>(`/api/rooms/${session.value.room.id}/me`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ nickname: nicknameDraft.value }) });
    session.value.room.members = session.value.room.members.map((member) => member.id === me.value?.id ? { ...member, nickname: result.nickname } : member);
    saveSession(session.value);
    editingNickname.value = false;
  } catch (error) { toast.error(errorMessage(error)); } finally { busy.value = false; }
}
function openPasswordDialog() { passwordDraft.value = ""; passwordOpen.value = true; }
async function saveRoomPassword() {
  if (!session.value) return;
  busy.value = true;
  try {
    const result = await request<{ has_password: boolean }>(`/api/rooms/${session.value.room.id}/password`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password: passwordDraft.value }) });
    session.value.room.has_password = result.has_password;
    saveSession(session.value);
    passwordOpen.value = false;
  } catch (error) { toast.error(error instanceof HTTPError && error.status === 400 ? t("rooms.passwordTooLong") : errorMessage(error)); } finally { busy.value = false; }
}
function closeSocket() { intentionalClose = true; connected.value = false; clearTimeout(reconnectTimer); clearInterval(pingTimer); socket?.close(); socket = null; }
function leaveRoom(remove = true) { if (remove && session.value) { savedRooms.value = savedRooms.value.filter((room) => room.id !== session.value?.room.id); localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value)); } closeSocket(); session.value = null; messages.value = []; void router.replace({ name: "rooms" }); }
function formatSize(bytes: number) { if (!bytes) return "0 B"; const units = ["B", "KB", "MB", "GB"]; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index] || "B"}`; }
function formatTime(value: string) { return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", month: "short", day: "numeric" }).format(new Date(value)); }
async function scrollBottom() { await nextTick(); const viewport = document.querySelector<HTMLElement>("[data-slot='scroll-area-viewport']"); if (viewport) viewport.scrollTop = viewport.scrollHeight; }
watch(session, (value) => setRoomFileDropHandler(value ? processRoomFile : null), { immediate: true });
onMounted(async () => {
  const id = String(route.params.roomID || ""); const saved = savedRooms.value.find((room) => room.id === id); if (saved) void openSavedRoom(saved);
  stopDesktopCopyListener = await listenForDesktopCopies((text) => {
    if (!connected.value || !session.value || !text.trim()) return;
    try { void setDesktopSharedText(text).catch(() => undefined); sendMessage({ kind: "text", source: "clipboard", text }); } catch (error) { toast.error(errorMessage(error)); }
  });
  if (isDesktopClient()) toast.info(t("rooms.desktopClipboardReady"));
});
onBeforeUnmount(() => { stopDesktopCopyListener?.(); void setDesktopSharedText(null); setRoomFileDropHandler(null); closeSocket(); });
</script>
