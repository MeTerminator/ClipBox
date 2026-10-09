const ball = document.getElementById("ball");
const progress = document.getElementById("progress");
const views = ["logo", "upload", "progress", "complete", "error"];
const idleTitle = "拖入文件上传，双击打开 ClipBox，右键隐藏";
let resetTimer;
let currentCode = null;
let stage = "idle";
let dragging = false;

function updateView() {
  const view = ["hashing", "uploading"].includes(stage) ? "progress"
    : stage === "error" ? "error"
    : ["complete", "shared"].includes(stage) ? "complete"
    : dragging ? "upload" : "logo";
  views.forEach((id) => { document.getElementById(id).hidden = id !== view; });
  ball.classList.toggle("dragging", dragging && stage === "idle");
  ball.classList.toggle("error", stage === "error");
}

function render(payload) {
  if (!payload) return;
  clearTimeout(resetTimer);
  currentCode = payload.code || null;
  stage = payload.stage;
  ball.title = payload.error || payload.filename || idleTitle;
  progress.textContent = `${Math.max(0, Math.min(100, Math.round(payload.progress || 0)))}%`;
  updateView();
  if (["complete", "shared", "error"].includes(stage)) {
    resetTimer = setTimeout(() => {
      stage = "idle";
      ball.title = idleTitle;
      updateView();
    }, 2200);
  }
}

const tauri = window.__TAURI__;
if (tauri?.event) {
  tauri.event.listen("floating://upload", ({ payload }) => render(payload));
  tauri.event.listen("floating://room-result", ({ payload }) => {
    if (payload.code === currentCode) render(payload);
  });
  tauri.event.listen("floating://drag", ({ payload }) => {
    dragging = Boolean(payload?.dragging);
    updateView();
  });
  ball.addEventListener("dblclick", () => tauri.core.invoke("show_main_window").catch(console.error));
  ball.addEventListener("contextmenu", (event) => {
    event.preventDefault();
    tauri.core.invoke("show_floating_menu").catch(console.error);
  });
  // Start moving only after a pointer gesture, preserving normal double clicks.
  let pointerStart;
  ball.addEventListener("pointerdown", (event) => {
    if (event.button === 0) pointerStart = { x: event.screenX, y: event.screenY };
  });
  ball.addEventListener("pointermove", (event) => {
    if (!pointerStart || !(event.buttons & 1)) return;
    if (Math.hypot(event.screenX - pointerStart.x, event.screenY - pointerStart.y) < 4) return;
    pointerStart = undefined;
    tauri.window.getCurrentWindow().startDragging().catch(console.error);
  });
  window.addEventListener("pointerup", () => { pointerStart = undefined; });
}
