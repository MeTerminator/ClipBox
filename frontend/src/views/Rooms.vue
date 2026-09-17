<template>
  <section class="rooms-page">
    <div v-if="!session" class="room-entry">
      <router-link :to="{ name: 'home' }" class="back-link"
        ><ArrowLeftIcon />{{ $t("rooms.back") }}</router-link
      >
      <div class="room-hero">
        <div class="room-mark"><MessagesSquareIcon /></div>
        <h1>{{ $t("rooms.title") }}</h1>
        <p>{{ $t("rooms.subtitle") }}</p>
      </div>
      <div class="entry-grid single-entry">
        <Transition name="entry-fade" mode="out-in">
          <form
            v-if="!showCreate"
            key="join"
            class="entry-card"
            @submit.prevent="joinRoom(joinForm.id)"
          >
            <h2>{{ $t("rooms.joinTitle") }}</h2>
            <label
              >{{ $t("rooms.roomID")
              }}<Input
                v-model="joinForm.id"
                maxlength="8"
                inputmode="numeric"
                pattern="[0-9]{8}"
                required
                class="room-code-input"
            /></label>
            <label
              >{{ $t("rooms.nickname")
              }}<Input
                v-model="joinForm.nickname"
                maxlength="40"
                :placeholder="$t('rooms.nicknameHint')"
            /></label>
            <Button :disabled="busy" type="submit" variant="outline"
              ><LogInIcon />{{ $t("rooms.join") }}</Button
            >
          </form>
          <form
            v-else
            key="create"
            class="entry-card"
            @submit.prevent="createRoom"
          >
            <h2>{{ $t("rooms.createTitle") }}</h2>
            <label
              >{{ $t("rooms.roomName")
              }}<Input
                v-model="createForm.name"
                maxlength="80"
                :placeholder="$t('rooms.roomNameOptional')"
            /></label>
            <label
              >{{ $t("rooms.nickname")
              }}<Input
                v-model="createForm.nickname"
                maxlength="40"
                :placeholder="$t('rooms.nicknameHint')"
            /></label>
            <Button :disabled="busy" type="submit"
              ><PlusIcon />{{ $t("rooms.create") }}</Button
            >
          </form>
        </Transition>
        <Button
          type="button"
          variant="ghost"
          class="create-room-toggle"
          @click="showCreate = !showCreate"
        >
          <LogInIcon v-if="showCreate" /><PlusIcon v-else />
          {{ showCreate ? $t("rooms.switchToJoin") : $t("rooms.create") }}
        </Button>
      </div>
      <div v-if="savedRooms.length" class="saved-rooms">
        <h2>{{ $t("rooms.savedRooms") }}</h2>
        <button
          v-for="room in savedRooms"
          :key="room.id"
          @click="openSavedRoom(room)"
        >
          <span
            ><strong>{{ room.name || $t("rooms.unnamed") }}</strong
            ><small>{{ room.id }} · {{ room.nickname }}</small></span
          ><ChevronRightIcon />
        </button>
      </div>
    </div>

    <div v-else class="room-shell">
      <header class="room-header">
        <div>
          <form
            v-if="editingName"
            class="room-name-editor"
            @submit.prevent="saveRoomName"
          >
            <Input v-model="roomNameDraft" maxlength="80" autofocus />
            <Button type="submit" size="icon" :disabled="busy"
              ><CheckIcon
            /></Button>
          </form>
          <h1 v-else>
            {{ session.room.name || $t("rooms.unnamed") }}
            <button
              v-if="me?.is_owner"
              class="rename-room"
              @click="startEditingName"
            >
              <PencilIcon />
            </button>
          </h1>
          <button class="room-id" @click="copyInvite">
            {{ session.room.id }} <CopyIcon />
          </button>
        </div>
        <div class="header-actions">
          <Button variant="outline" size="sm" @click="copyInvite"
            ><LinkIcon />{{ $t("rooms.copyLink") }}</Button
          >
          <Button
            v-if="me?.is_owner"
            variant="outline"
            size="sm"
            @click="deleteRoom"
            ><Trash2Icon />{{ $t("rooms.delete") }}</Button
          >
          <Button variant="ghost" size="sm" @click="leaveRoom"
            ><LogOutIcon />{{ $t("rooms.leave") }}</Button
          >
        </div>
      </header>
      <div class="room-layout">
        <main class="conversation">
          <div ref="messageList" class="message-list">
            <div v-if="!messages.length" class="empty-chat">
              <MessagesSquareIcon />
              <p>{{ $t("rooms.empty") }}</p>
            </div>
            <article
              v-for="message in messages"
              :key="message.id"
              :class="[
                'message',
                { mine: message.sender.id === session.room.current_member_id },
              ]"
            >
              <div class="message-meta">
                <strong>{{ message.sender.nickname }}</strong
                ><span>{{ formatTime(message.created_at) }}</span
                ><span
                  v-if="message.source === 'clipboard'"
                  class="source-pill"
                  >{{ $t("rooms.clipboard") }}</span
                >
              </div>
              <div v-if="message.kind === 'text'" class="bubble">
                {{ message.text }}
              </div>
              <button v-else class="file-bubble" @click="downloadFile(message)">
                <span class="file-icon"><FileIcon /></span
                ><span
                  ><strong>{{ message.file.name }}</strong
                  ><small>{{ formatSize(message.file.size) }}</small></span
                ><DownloadIcon />
              </button>
            </article>
          </div>
          <form class="composer" @submit.prevent="sendText">
            <input ref="fileInput" type="file" hidden @change="sendFile" />
            <Button
              type="button"
              variant="outline"
              size="icon"
              :title="$t('rooms.attach')"
              :disabled="busy || !connected"
              @click="fileInput.click()"
              ><PaperclipIcon
            /></Button>
            <Textarea
              v-model="draft"
              :placeholder="$t('rooms.sendPlaceholder')"
              @keydown.enter.exact.prevent="sendText"
            />
            <Button
              type="submit"
              size="icon"
              :disabled="busy || !connected || !draft.trim()"
              ><SendIcon
            /></Button>
          </form>
          <div v-if="uploadStage" class="upload-line">
            <span :style="{ width: `${uploadProgress}%` }" />
          </div>
        </main>
        <aside class="member-panel">
          <h2>
            {{ $t("rooms.members") }}
            <span>{{ session.room.members.length }}</span>
          </h2>
          <div
            v-for="member in session.room.members"
            :key="member.id"
            class="member"
          >
            <div class="avatar">
              {{ member.nickname.slice(0, 1).toUpperCase() }}
            </div>
            <div>
              <strong
                >{{ member.nickname }} <CrownIcon v-if="member.is_owner"
              /></strong>
              <p>{{ $t("rooms.deviceInfo", member) }}</p>
              <small :class="{ online: member.online }"
                ><i />{{
                  member.online ? $t("rooms.online") : $t("rooms.offline")
                }}</small
              >
            </div>
          </div>
        </aside>
      </div>
    </div>
  </section>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useFileUpload } from "@/composables/useFileUpload";
import { setRoomFileDropHandler } from "@/composables/fileDropTarget";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import {
  ArrowLeftIcon,
  CheckIcon,
  ChevronRightIcon,
  CopyIcon,
  CrownIcon,
  DownloadIcon,
  FileIcon,
  LinkIcon,
  LogInIcon,
  LogOutIcon,
  MessagesSquareIcon,
  PaperclipIcon,
  PencilIcon,
  PlusIcon,
  SendIcon,
  Trash2Icon,
} from "lucide-vue-next";

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const createForm = reactive({
  name: localStorage.getItem("lastRoomName") || "",
  nickname: "",
});
const joinForm = reactive({
  id: String(route.params.roomID || ""),
  nickname: "",
});
const session = ref(null);
const messages = ref([]);
const draft = ref("");
const busy = ref(false);
const connected = ref(false);
const fileInput = ref(null);
const messageList = ref(null);
const showCreate = ref(false);
const editingName = ref(false);
const roomNameDraft = ref("");
const { uploadStage, uploadProgress, uploadFile, resetUploadProgress } =
  useFileUpload();
const savedRooms = ref(loadSavedRooms());
let socket = null;
let reconnectTimer = null;
let pingTimer = null;
let intentionalClose = false;
const me = computed(() =>
  session.value?.room.members.find(
    (member) => member.id === session.value.room.current_member_id,
  ),
);

function loadSavedRooms() {
  try {
    return JSON.parse(localStorage.getItem("shareRooms") || "[]");
  } catch {
    return [];
  }
}
function saveSession(value) {
  const entry = {
    id: value.room.id,
    name: value.room.name,
    nickname:
      value.room.members.find((m) => m.id === value.room.current_member_id)
        ?.nickname || "",
    token: value.token,
  };
  savedRooms.value = [
    entry,
    ...savedRooms.value.filter((r) => r.id !== entry.id),
  ].slice(0, 20);
  localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
}
function deviceInfo() {
  const ua = navigator.userAgent;
  let os = "Unknown OS",
    browser = "Unknown browser",
    device = /Mobile|Android|iPhone|iPad/i.test(ua) ? "Mobile" : "Desktop";
  if (/iPhone/.test(ua)) {
    device = "iPhone";
    os = "iOS";
  } else if (/iPad/.test(ua)) {
    device = "iPad";
    os = "iPadOS";
  } else if (/Android/.test(ua)) os = "Android";
  else if (/Windows/.test(ua)) os = "Windows";
  else if (/Mac OS X/.test(ua)) os = "macOS";
  else if (/Linux/.test(ua)) os = "Linux";
  if (/Edg\//.test(ua)) browser = "Edge";
  else if (/Firefox\//.test(ua)) browser = "Firefox";
  else if (/Chrome\//.test(ua)) browser = "Chrome";
  else if (/Safari\//.test(ua)) browser = "Safari";
  return { device, os, browser };
}
async function request(url, options = {}, token = session.value?.token) {
  const headers = { ...(options.headers || {}) };
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(url, { ...options, headers });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    const error = new Error(body.error || t("rooms.unknownError"));
    error.status = response.status;
    throw error;
  }
  if (response.status === 204) return null;
  return response.json();
}
async function createRoom() {
  busy.value = true;
  try {
    const data = await request(
      "/api/rooms",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...createForm, ...deviceInfo() }),
      },
      null,
    );
    activate(data);
  } catch (e) {
    toast.error(e.message);
  } finally {
    busy.value = false;
  }
}
async function joinRoom(id) {
  busy.value = true;
  try {
    const normalized = String(id).trim().toUpperCase();
    const data = await request(
      `/api/rooms/${normalized}/join`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ nickname: joinForm.nickname, ...deviceInfo() }),
      },
      null,
    );
    activate(data);
  } catch (e) {
    toast.error(e.status === 404 ? t("rooms.roomNotFound") : e.message);
  } finally {
    busy.value = false;
  }
}
function activate(data) {
  closeSocket();
  session.value = data;
  messages.value = [];
  saveSession(data);
  router.replace({ name: "rooms", params: { roomID: data.room.id } });
  connectSocket();
}
async function openSavedRoom(room) {
  busy.value = true;
  try {
    const info = await request(`/api/rooms/${room.id}`, {}, room.token);
    activate({ room: info, token: room.token });
  } catch {
    savedRooms.value = savedRooms.value.filter((r) => r.id !== room.id);
    localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
    toast.error(t("rooms.roomNotFound"));
  } finally {
    busy.value = false;
  }
}
function connectSocket() {
  if (!session.value) return;
  connected.value = false;
  intentionalClose = false;
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  const connection = (socket = new WebSocket(
    `${scheme}://${location.host}/api/rooms/${session.value.room.id}/ws`,
  ));
  connection.addEventListener("open", () => {
    connection.send(
      JSON.stringify({ type: "auth", token: session.value.token }),
    );
    clearInterval(pingTimer);
    pingTimer = setInterval(() => sendSocket({ type: "ping" }), 20000);
  });
  connection.addEventListener("message", handleSocketEvent);
  connection.addEventListener("close", () => {
    if (connection !== socket) return;
    connected.value = false;
    clearInterval(pingTimer);
    if (!intentionalClose && session.value) {
      clearTimeout(reconnectTimer);
      reconnectTimer = setTimeout(connectSocket, 1500);
    }
  });
  connection.addEventListener("error", () => connection.close());
}
async function handleSocketEvent(event) {
  let data;
  try {
    data = JSON.parse(event.data);
  } catch {
    return;
  }
  if (data.type === "ready") {
    connected.value = true;
    session.value.room = data.room;
    messages.value = data.messages || [];
    await scrollBottom();
  } else if (data.type === "message") {
    if (!messages.value.some((item) => item.id === data.message.id))
      messages.value.push(data.message);
    await scrollBottom();
  } else if (data.type === "presence") {
    session.value.room.members = data.members || [];
  } else if (data.type === "room_updated") {
    session.value.room.name = data.name;
    saveSession(session.value);
  } else if (data.type === "room_deleted") {
    toast.error(t("rooms.roomNotFound"));
    leaveRoom();
  } else if (data.type === "error") {
    toast.error(t("rooms.unknownError"));
  }
}
function sendSocket(command) {
  if (!socket || socket.readyState !== WebSocket.OPEN)
    throw new Error(t("rooms.unknownError"));
  socket.send(JSON.stringify(command));
}
function sendMessage(payload) {
  sendSocket({ type: "send", message: payload });
}
function sendText() {
  const text = draft.value.trim();
  if (!text || busy.value) return;
  try {
    sendMessage({ kind: "text", source: "user", text });
    draft.value = "";
  } catch (e) {
    toast.error(e.message);
  }
}
async function processRoomFile(file) {
  if (!file || busy.value) return;
  if (!connected.value) {
    toast.error(t("rooms.unknownError"));
    return;
  }
  busy.value = true;
  resetUploadProgress();
  try {
    const result = await uploadFile(file, { count: 1000, expire: 31536000 });
    sendMessage({ kind: "file", source: "user", file_code: result.code });
  } catch (e) {
    toast.error(e.message);
  } finally {
    busy.value = false;
    resetUploadProgress();
  }
}
async function sendFile(event) {
  const file = event.target.files?.[0];
  event.target.value = "";
  await processRoomFile(file);
}
async function downloadFile(message) {
  try {
    const response = await fetch(message.file.download_url, {
      headers: { Authorization: `Bearer ${session.value.token}` },
    });
    if (!response.ok) throw new Error(t("rooms.unknownError"));
    const url = URL.createObjectURL(await response.blob());
    const link = document.createElement("a");
    link.href = url;
    link.download = message.file.name;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  } catch (e) {
    toast.error(e.message);
  }
}
async function copyInvite() {
  await navigator.clipboard.writeText(
    `${location.origin}/rooms/${session.value.room.id}`,
  );
  toast.success(t("rooms.copied"));
}
async function deleteRoom() {
  if (!confirm(t("rooms.deleteConfirm"))) return;
  try {
    await request(`/api/rooms/${session.value.room.id}`, { method: "DELETE" });
    leaveRoom();
  } catch (e) {
    toast.error(e.status === 403 ? t("rooms.onlyOwner") : e.message);
  }
}
function startEditingName() {
  roomNameDraft.value = session.value.room.name || "";
  editingName.value = true;
}
async function saveRoomName() {
  busy.value = true;
  try {
    const result = await request(`/api/rooms/${session.value.room.id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: roomNameDraft.value }),
    });
    session.value.room.name = result.name;
    localStorage.setItem("lastRoomName", result.name);
    editingName.value = false;
  } catch (e) {
    toast.error(e.message);
  } finally {
    busy.value = false;
  }
}
function closeSocket() {
  intentionalClose = true;
  connected.value = false;
  clearTimeout(reconnectTimer);
  clearInterval(pingTimer);
  socket?.close();
  socket = null;
}
function leaveRoom(remove = true) {
  if (remove && session.value) {
    savedRooms.value = savedRooms.value.filter(
      (r) => r.id !== session.value.room.id,
    );
    localStorage.setItem("shareRooms", JSON.stringify(savedRooms.value));
  }
  closeSocket();
  session.value = null;
  messages.value = [];
  router.replace({ name: "rooms" });
}
function formatSize(bytes) {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1,
  );
  return `${(bytes / 1024 ** i).toFixed(i ? 1 : 0)} ${units[i]}`;
}
function formatTime(value) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
    day: "numeric",
  }).format(new Date(value));
}
async function scrollBottom() {
  await nextTick();
  if (messageList.value)
    messageList.value.scrollTop = messageList.value.scrollHeight;
}
watch(
  () => createForm.name,
  (name) => localStorage.setItem("lastRoomName", name),
);
watch(
  session,
  (value) => setRoomFileDropHandler(value ? processRoomFile : null),
  { immediate: true },
);
onMounted(() => {
  const id = String(route.params.roomID || "");
  const saved = savedRooms.value.find((r) => r.id === id);
  if (saved) openSavedRoom(saved);
});
onBeforeUnmount(() => {
  setRoomFileDropHandler(null);
  closeSocket();
});
</script>

<style scoped>
.rooms-page {
  min-height: calc(100vh - 180px);
}
.back-link {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: hsl(var(--muted-foreground));
  font-size: 0.9rem;
  text-decoration: none;
}
.back-link svg {
  width: 1rem;
}
.room-hero {
  text-align: center;
  margin: 2.5rem 0;
}
.room-mark {
  width: 3.5rem;
  height: 3.5rem;
  margin: auto;
  display: grid;
  place-items: center;
  border: 1px solid hsl(var(--border) / 0.4);
  border-radius: 1rem;
}
.room-mark svg {
  width: 1.6rem;
}
.room-hero h1 {
  font-size: 2.25rem;
  font-weight: 800;
  margin: 0.9rem 0 0.35rem;
}
.room-hero p {
  color: hsl(var(--muted-foreground));
}
.entry-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}
.entry-card {
  border: 1px solid hsl(var(--border) / 0.35);
  border-radius: 1rem;
  padding: 1.25rem;
  display: grid;
  gap: 1rem;
}
.entry-card h2 {
  font-weight: 750;
  font-size: 1.08rem;
}
.entry-card label {
  display: grid;
  gap: 0.45rem;
  font-size: 0.8rem;
  color: hsl(var(--muted-foreground));
  font-weight: 650;
}
.entry-card button {
  gap: 0.4rem;
}
.entry-card button svg {
  width: 1rem;
}
.room-code-input {
  text-transform: uppercase;
  font-family: ui-monospace, monospace;
  letter-spacing: 0.08em;
}
.saved-rooms {
  margin-top: 1.5rem;
}
.saved-rooms h2 {
  font-weight: 700;
  margin-bottom: 0.6rem;
}
.saved-rooms > button {
  width: 100%;
  border-top: 1px solid hsl(var(--border) / 0.25);
  padding: 0.9rem 0.2rem;
  display: flex;
  justify-content: space-between;
  text-align: left;
}
.saved-rooms span {
  display: grid;
}
.saved-rooms small {
  color: hsl(var(--muted-foreground));
  font-family: ui-monospace, monospace;
}
.saved-rooms svg {
  width: 1rem;
}
.room-shell {
  border: 1px solid hsl(var(--border) / 0.3);
  border-radius: 1rem;
  overflow: hidden;
  min-height: 680px;
}
.room-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.2rem;
  border-bottom: 1px solid hsl(var(--border) / 0.3);
}
.room-header h1 {
  font-weight: 800;
  font-size: 1.25rem;
}
.room-id {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: hsl(var(--muted-foreground));
  font:
    700 0.72rem ui-monospace,
    monospace;
}
.room-id svg {
  width: 0.75rem;
}
.header-actions {
  display: flex;
  gap: 0.45rem;
}
.header-actions button {
  gap: 0.35rem;
}
.header-actions svg {
  width: 0.9rem;
}
.room-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 220px;
  min-height: 620px;
}
.conversation {
  display: grid;
  grid-template-rows: 1fr auto auto;
  min-width: 0;
}
.message-list {
  height: 540px;
  overflow-y: auto;
  padding: 1.25rem;
}
.empty-chat {
  height: 100%;
  display: grid;
  place-content: center;
  justify-items: center;
  color: hsl(var(--muted-foreground));
  gap: 0.7rem;
  font-size: 0.9rem;
}
.empty-chat svg {
  width: 2rem;
}
.message {
  margin-bottom: 1rem;
  max-width: 80%;
}
.message.mine {
  margin-left: auto;
}
.message-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.72rem;
  color: hsl(var(--muted-foreground));
  margin: 0 0 0.3rem 0.15rem;
}
.message.mine .message-meta {
  justify-content: flex-end;
}
.message-meta strong {
  color: hsl(var(--foreground));
  font-size: 0.78rem;
}
.source-pill {
  border: 1px solid hsl(var(--border) / 0.35);
  border-radius: 99px;
  padding: 0.1rem 0.4rem;
}
.bubble {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  border: 1px solid hsl(var(--border) / 0.3);
  border-radius: 0.85rem 0.85rem 0.85rem 0.2rem;
  padding: 0.7rem 0.85rem;
  background: hsl(var(--muted) / 0.45);
}
.mine .bubble {
  background: hsl(var(--foreground));
  color: hsl(var(--background));
  border-radius: 0.85rem 0.85rem 0.2rem 0.85rem;
}
.file-bubble {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  min-width: 260px;
  border: 1px solid hsl(var(--border) / 0.3);
  border-radius: 0.85rem;
  padding: 0.75rem;
  text-align: left;
}
.file-bubble > span:nth-child(2) {
  display: grid;
  min-width: 0;
  flex: 1;
}
.file-bubble strong {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.file-bubble small {
  color: hsl(var(--muted-foreground));
}
.file-bubble > svg {
  width: 1rem;
}
.file-icon {
  width: 2.2rem;
  height: 2.2rem;
  border-radius: 0.55rem;
  background: hsl(var(--muted));
  display: grid;
  place-items: center;
}
.file-icon svg {
  width: 1.1rem;
}
.composer {
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr) 48px;
  align-items: stretch;
  gap: 0.4rem;
  padding: 0.8rem;
  border-top: 1px solid hsl(var(--border) / 0.3);
}
.composer textarea {
  width: 100%;
  min-height: 48px;
  height: 48px;
  max-height: 120px;
  line-height: 30px;
  resize: none;
}
.composer > button {
  width: 48px;
  height: 48px;
  align-self: end;
}
.composer > button svg {
  width: 1rem;
}
.upload-line {
  height: 2px;
  background: hsl(var(--muted));
}
.upload-line span {
  height: 100%;
  display: block;
  background: hsl(var(--foreground));
  transition: width 0.2s;
}
.member-panel {
  border-left: 1px solid hsl(var(--border) / 0.3);
  padding: 1rem;
}
.member-panel h2 {
  font-size: 0.8rem;
  font-weight: 750;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: hsl(var(--muted-foreground));
  margin-bottom: 1rem;
}
.member-panel h2 span {
  float: right;
}
.member {
  display: flex;
  gap: 0.65rem;
  margin-bottom: 1rem;
}
.avatar {
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  border-radius: 0.65rem;
  background: hsl(var(--muted));
  font-weight: 800;
}
.member > div:last-child {
  min-width: 0;
}
.member strong {
  font-size: 0.82rem;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
.member strong svg {
  width: 0.75rem;
}
.member p {
  font-size: 0.67rem;
  color: hsl(var(--muted-foreground));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 155px;
}
.member small {
  font-size: 0.65rem;
  color: hsl(var(--muted-foreground));
  display: flex;
  align-items: center;
  gap: 0.3rem;
}
.member small i {
  width: 0.35rem;
  height: 0.35rem;
  border-radius: 50%;
  background: hsl(var(--muted-foreground));
}
.member small.online i {
  background: #22c55e;
}
.single-entry {
  grid-template-columns: 1fr;
  max-width: 430px;
  margin: 0 auto;
}
.create-room-toggle {
  gap: 0.45rem;
}
.create-room-toggle svg {
  width: 1rem;
}
.entry-fade-enter-active,
.entry-fade-leave-active {
  transition:
    opacity 180ms ease,
    transform 180ms ease;
}
.entry-fade-enter-from,
.entry-fade-leave-to {
  opacity: 0;
  transform: translateY(5px);
}
.room-name-editor {
  display: flex;
  gap: 0.4rem;
  align-items: center;
}
.room-name-editor input {
  max-width: 280px;
}
.room-name-editor button svg {
  width: 1rem;
}
.rename-room {
  display: inline-flex;
  vertical-align: middle;
  margin-left: 0.25rem;
  color: hsl(var(--muted-foreground));
}
.rename-room svg {
  width: 0.85rem;
}
@media (max-width: 700px) {
  .entry-grid {
    grid-template-columns: 1fr;
  }
  .room-shell {
    border-radius: 0.75rem;
  }
  .room-header {
    align-items: flex-start;
  }
  .header-actions button {
    font-size: 0;
    padding: 0.5rem;
  }
  .header-actions button svg {
    width: 1rem;
    margin: 0;
  }
  .room-layout {
    grid-template-columns: 1fr;
  }
  .member-panel {
    grid-row: 1;
    border-left: 0;
    border-bottom: 1px solid hsl(var(--border) / 0.3);
    display: flex;
    gap: 0.7rem;
    overflow-x: auto;
  }
  .member-panel h2 {
    display: none;
  }
  .member {
    min-width: 150px;
    margin: 0;
  }
  .message-list {
    height: 500px;
  }
  .message {
    max-width: 90%;
  }
  .file-bubble {
    min-width: 220px;
  }
}
</style>
