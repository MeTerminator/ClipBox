const ball = document.getElementById("ball");
const name = document.getElementById("name");
const status = document.getElementById("status");
let resetTimer;
let currentCode = null;

function render(payload) {
  if (!payload) return;
  clearTimeout(resetTimer);
  currentCode = payload.code || null;
  name.textContent = payload.filename || "文件";
  ball.title = payload.error || payload.filename || "拖入文件快速上传";
  status.textContent =
    payload.stage === "hashing"
      ? `${payload.progress}%`
      : payload.stage === "uploading"
        ? `${payload.progress}%`
        : payload.stage === "complete"
          ? (payload.to_room ? "已上传" : (payload.code || "完成"))
          : payload.stage === "shared"
            ? "已发送"
          : payload.stage === "error"
            ? "失败"
            : "ClipBox";
  ball.style.setProperty("--progress", String(payload.progress || 0));
  ball.style.setProperty("--ring-opacity", ["complete", "shared"].includes(payload.stage) ? "0" : "1");
  ball.classList.toggle("error", payload.stage === "error");
  if (["complete", "shared", "error"].includes(payload.stage)) {
    resetTimer = setTimeout(() => {
      name.textContent = "拖入文件";
      status.textContent = "ClipBox";
      ball.style.setProperty("--progress", "0");
      ball.style.setProperty("--ring-opacity", "0");
      ball.classList.remove("error");
    }, 2200);
  }
}

if (window.__TAURI__?.event) {
  window.__TAURI__.event.listen("floating://upload", ({ payload }) => render(payload));
  window.__TAURI__.event.listen("floating://room-result", ({ payload }) => {
    if (payload.code === currentCode) render(payload);
  });
  window.__TAURI__.event.listen("floating://drag", ({ payload }) => {
    ball.classList.toggle("dragging", Boolean(payload?.dragging));
  });
}
