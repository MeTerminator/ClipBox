use std::{
    fs::{create_dir_all, File, OpenOptions},
    io::{Read, Seek, SeekFrom, Write},
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};

use arboard::Clipboard;
#[cfg(not(target_os = "macos"))]
use rdev::{listen, Event, EventType, Key};

#[cfg(target_os = "macos")]
mod macos_keyboard;
use serde::{Deserialize, Serialize};
use sha1::{Digest, Sha1};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    DragDropEvent, Emitter, Manager, State, WebviewUrl, WebviewWindowBuilder, WindowEvent,
};

const DEFAULT_BACKEND_ORIGIN: &str = "http://127.0.0.1:5328";
const HASH_CHUNK_SIZE: usize = 4 * 1024 * 1024;
const UPLOAD_WORKERS: usize = 4;

#[derive(Default)]
struct ClipboardState {
    enabled: bool,
    text: Option<String>,
}

impl ClipboardState {
    fn set_enabled(&mut self, enabled: bool) {
        self.enabled = enabled;
        self.text = None;
    }

    fn cache_text(&mut self, text: Option<String>) {
        self.text = text.filter(|value| self.enabled && !value.is_empty());
    }
}

#[derive(Clone)]
struct DesktopRuntime {
    shared_text: Arc<Mutex<ClipboardState>>,
    backend_origin: Arc<Mutex<String>>,
    room_id: Arc<Mutex<Option<String>>>,
    download_origins: Arc<Mutex<std::collections::HashSet<String>>>,
    upload_queue: Arc<tokio::sync::Mutex<()>>,
}

impl Default for DesktopRuntime {
    fn default() -> Self {
        Self {
            shared_text: Arc::new(Mutex::new(ClipboardState::default())),
            backend_origin: Arc::new(Mutex::new(DEFAULT_BACKEND_ORIGIN.to_string())),
            room_id: Arc::new(Mutex::new(None)),
            download_origins: Arc::new(Mutex::new(Default::default())),
            upload_queue: Arc::new(tokio::sync::Mutex::new(())),
        }
    }
}

impl DesktopRuntime {
    fn backend_origin(&self) -> String {
        self.backend_origin
            .lock()
            .map(|value| value.clone())
            .unwrap_or_else(|_| DEFAULT_BACKEND_ORIGIN.to_string())
    }

    fn room_id(&self) -> Option<String> {
        self.room_id.lock().ok().and_then(|value| value.clone())
    }
}

#[derive(Clone, Serialize)]
struct ClipboardCopyPayload {
    text: String,
}

#[derive(Clone, Serialize)]
struct FloatingDragPayload {
    dragging: bool,
}

#[derive(Clone, Serialize)]
struct FloatingUploadPayload {
    stage: String,
    filename: String,
    progress: u8,
    code: Option<String>,
    to_room: bool,
    room_id: Option<String>,
    error: Option<String>,
}

#[cfg(not(target_os = "macos"))]
#[derive(Default)]
struct KeyState {
    ctrl: bool,
    meta: bool,
    c: bool,
    v: bool,
}

#[tauri::command]
fn set_shared_text(runtime: State<'_, DesktopRuntime>, text: Option<String>) {
    if let Ok(mut current) = runtime.shared_text.lock() {
        current.cache_text(text);
        #[cfg(target_os = "macos")]
        eprintln!(
            "[clipboard] room text cache: {}",
            if current.text.is_some() {
                "ready"
            } else {
                "empty"
            }
        );
    }
}

#[tauri::command]
fn set_clipboard_sharing(runtime: State<'_, DesktopRuntime>, enabled: bool) {
    if let Ok(mut clipboard) = runtime.shared_text.lock() {
        clipboard.set_enabled(enabled);
    }
}

#[tauri::command]
fn show_main_window(app: tauri::AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window.show().map_err(|error| error.to_string())?;
        window.unminimize().map_err(|error| error.to_string())?;
        window.set_focus().map_err(|error| error.to_string())?;
    }
    Ok(())
}

#[tauri::command]
fn show_floating_menu(window: tauri::WebviewWindow) -> tauri::Result<()> {
    let hide = MenuItem::with_id(&window, "hide-floating", "隐藏悬浮窗", true, None::<&str>)?;
    let menu = Menu::with_items(&window, &[&hide])?;
    window.popup_menu(&menu)
}

fn http_url(value: &str) -> Result<reqwest::Url, String> {
    let url = reqwest::Url::parse(value).map_err(|_| "invalid URL")?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.fragment().is_some()
    {
        return Err("URL must use HTTP(S), without credentials or fragment".into());
    }
    Ok(url)
}

fn network_client() -> Result<reqwest::Client, String> {
    reqwest::Client::builder()
        // Never forward room tokens or uploaded file bodies through redirects.
        .redirect(reqwest::redirect::Policy::none())
        .connect_timeout(Duration::from_secs(15))
        .read_timeout(Duration::from_secs(60))
        .build()
        .map_err(|error| error.to_string())
}

#[tauri::command]
fn set_backend_origin(runtime: State<'_, DesktopRuntime>, origin: String) -> Result<(), String> {
    let value = origin.trim();
    let url = http_url(if value.is_empty() {
        DEFAULT_BACKEND_ORIGIN
    } else {
        value
    })?;
    if url.path() != "/" || url.query().is_some() {
        return Err("backend URL must be an origin".into());
    }
    let origin = url.origin().ascii_serialization();
    // The frontend registers the primary backend, then its direct-transfer origin.
    runtime
        .download_origins
        .lock()
        .map_err(|_| "runtime lock failed")?
        .insert(origin.clone());
    *runtime
        .backend_origin
        .lock()
        .map_err(|_| "runtime lock failed")? = origin;
    Ok(())
}

fn validate_download_url(
    value: &str,
    origins: &std::collections::HashSet<String>,
) -> Result<reqwest::Url, String> {
    let url = http_url(value)?;
    if !origins.contains(&url.origin().ascii_serialization()) {
        return Err("download origin has not been registered".into());
    }
    let parts: Vec<_> = url.path().split('/').collect();
    if parts.len() != 7
        || parts[1] != "api"
        || parts[2] != "rooms"
        || parts[3].len() != 5
        || !parts[3].bytes().all(|c| c.is_ascii_digit())
        || parts[4] != "messages"
        || parts[5].is_empty()
        || !parts[5].bytes().all(|c| c.is_ascii_digit())
        || parts[6] != "file"
        || url.query().is_some()
    {
        return Err("invalid room file URL".into());
    }
    Ok(url)
}

#[tauri::command]
fn set_upload_target(runtime: State<'_, DesktopRuntime>, room_id: Option<String>) {
    if let Ok(mut current) = runtime.room_id.lock() {
        if *current != room_id {
            if let Ok(mut text) = runtime.shared_text.lock() {
                text.text = None;
            }
        }
        *current = room_id;
    }
}

#[tauri::command]
fn save_portable_storage(
    storage: std::collections::BTreeMap<String, String>,
) -> Result<(), String> {
    static STORAGE_WRITE: Mutex<()> = Mutex::new(());
    let _write = STORAGE_WRITE.lock().map_err(|_| "storage lock failed")?;
    let directory = create_dir_under_executable("data")?;
    let bytes = serde_json::to_vec(&storage).map_err(|error| error.to_string())?;
    let pending = directory.join("local-storage.json.tmp");
    let mut options = OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options.open(&pending).map_err(|error| error.to_string())?;
    file.write_all(&bytes).map_err(|error| error.to_string())?;
    file.sync_all().map_err(|error| error.to_string())?;
    drop(file);
    std::fs::rename(pending, directory.join("local-storage.json"))
        .map_err(|error| error.to_string())
}

fn executable_directory() -> PathBuf {
    std::env::current_exe()
        .ok()
        .and_then(|path| path.parent().map(Path::to_path_buf))
        .unwrap_or_else(|| PathBuf::from("."))
}

fn create_dir_under_executable(name: &str) -> Result<PathBuf, String> {
    let path = executable_directory().join(name);
    create_dir_all(&path).map_err(|error| format!("failed to create {name}: {error}"))?;
    Ok(path)
}

fn safe_filename(value: &str) -> String {
    let file_name = Path::new(value)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("download");
    let cleaned: String = file_name
        .chars()
        .map(|character| match character {
            '\\' | '/' | ':' | '*' | '?' | '"' | '<' | '>' | '|' => '_',
            character if character.is_control() => '_',
            character => character,
        })
        .scan(0, |length, character| {
            *length += character.len_utf8();
            (*length <= 180).then_some(character)
        })
        .collect();
    let cleaned = cleaned.trim().trim_end_matches(['.', ' ']);
    let stem = cleaned
        .split('.')
        .next()
        .unwrap_or_default()
        .to_ascii_uppercase();
    let reserved = matches!(stem.as_str(), "CON" | "PRN" | "AUX" | "NUL")
        || (stem.len() == 4
            && (stem.starts_with("COM") || stem.starts_with("LPT"))
            && matches!(stem.as_bytes()[3], b'1'..=b'9'));
    if cleaned.is_empty() {
        "download".to_string()
    } else if reserved {
        format!("_{cleaned}")
    } else {
        cleaned.to_string()
    }
}

fn create_unique_download_file(
    directory: &Path,
    filename: &str,
) -> Result<(PathBuf, File), String> {
    let safe_name = safe_filename(filename);
    let source = Path::new(&safe_name);
    let stem = source
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("download");
    let extension = source
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default();
    let now = chrono::Local::now();
    let timestamp = now.format("%Y%m%d-%H%M%S");
    for attempt in 0..1000_u32 {
        let name = if attempt == 0 {
            safe_name.clone()
        } else {
            let suffix = if attempt == 1 {
                timestamp.to_string()
            } else {
                format!("{timestamp}-{attempt}")
            };
            if extension.is_empty() {
                format!("{stem}-{suffix}")
            } else {
                format!("{stem}-{suffix}.{extension}")
            }
        };
        let path = directory.join(name);
        match OpenOptions::new().write(true).create_new(true).open(&path) {
            Ok(file) => return Ok((path, file)),
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => continue,
            Err(error) => return Err(format!("save failed: {error}")),
        }
    }

    Err("could not create a unique download filename".to_string())
}

#[tauri::command]
async fn save_room_file(
    runtime: State<'_, DesktopRuntime>,
    url: String,
    token: Option<String>,
    filename: String,
) -> Result<String, String> {
    let url = {
        let origins = runtime
            .download_origins
            .lock()
            .map_err(|_| "runtime lock failed")?;
        validate_download_url(&url, &origins)?
    };
    let directory = create_dir_under_executable("downloads")?;
    let client = network_client()?;
    let mut request = client.get(url);
    if let Some(token) = token {
        request = request.bearer_auth(token);
    }
    let mut response = request
        .send()
        .await
        .map_err(|error| format!("download failed: {error}"))?;
    if !response.status().is_success() {
        return Err(format!("download failed ({})", response.status()));
    }
    let (path, mut file) = create_unique_download_file(&directory, &filename)?;
    let result = async {
        while let Some(bytes) = response
            .chunk()
            .await
            .map_err(|error| format!("download failed: {error}"))?
        {
            file.write_all(&bytes)
                .map_err(|error| format!("save failed: {error}"))?;
        }
        file.sync_all()
            .map_err(|error| format!("save failed: {error}"))
    }
    .await;
    drop(file);
    if let Err(error) = result {
        let _ = std::fs::remove_file(&path);
        return Err(error);
    }
    Ok(path.display().to_string())
}

#[derive(Serialize)]
struct UploadInitRequest<'a> {
    filename: &'a str,
    size: u64,
    sha1: &'a str,
    count: u32,
    expire: u32,
}

#[derive(Deserialize)]
struct UploadInitResponse {
    #[serde(default)]
    instant_upload: bool,
    #[serde(default)]
    code: Option<String>,
    #[serde(default)]
    upload_id: Option<String>,
    #[serde(default)]
    chunk_size: Option<u64>,
    #[serde(default)]
    total_chunks: Option<usize>,
    #[serde(default)]
    workers: Option<usize>,
    #[serde(default)]
    uploaded_chunks: Option<Vec<usize>>,
}

#[derive(Deserialize)]
struct UploadCompleteResponse {
    #[serde(default)]
    code: Option<String>,
}

#[derive(Serialize)]
struct UploadCompleteRequest<'a> {
    filename: &'a str,
    count: u32,
    expire: u32,
}

fn emit_upload(
    app: &tauri::AppHandle,
    stage: &str,
    filename: &str,
    progress: u8,
    code: Option<String>,
    room_id: Option<String>,
    error: Option<String>,
) {
    let _ = app.emit(
        "floating://upload",
        FloatingUploadPayload {
            stage: stage.to_string(),
            filename: filename.to_string(),
            progress,
            code,
            to_room: room_id.is_some(),
            room_id,
            error,
        },
    );
}

fn sha1_hex(digest: [u8; 20]) -> String {
    let mut output = String::with_capacity(40);
    for byte in digest {
        output.push_str(&format!("{byte:02x}"));
    }
    output
}

fn hash_file(
    app: &tauri::AppHandle,
    path: &Path,
    filename: &str,
    room_id: Option<String>,
) -> Result<(u64, String), String> {
    emit_upload(app, "hashing", filename, 0, None, room_id.clone(), None);
    let file = File::open(path).map_err(|error| format!("open file failed: {error}"))?;
    let metadata = file
        .metadata()
        .map_err(|error| format!("read file metadata failed: {error}"))?;
    if !metadata.is_file() {
        return Err("only regular files can be uploaded".into());
    }
    let total = metadata.len().max(1);
    let mut file = file;
    let mut hasher = Sha1::new();
    let mut buffer = vec![0_u8; HASH_CHUNK_SIZE];
    let mut read_total = 0_u64;

    loop {
        let read = file
            .read(&mut buffer)
            .map_err(|error| format!("read file failed: {error}"))?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
        read_total += read as u64;
        let progress = ((read_total * 100) / total).min(100) as u8;
        emit_upload(
            app,
            "hashing",
            filename,
            progress,
            None,
            room_id.clone(),
            None,
        );
    }

    Ok((read_total, sha1_hex(hasher.finalize().into())))
}

fn read_chunk(path: &Path, start: u64, length: usize) -> Result<Vec<u8>, String> {
    let mut file = File::open(path).map_err(|error| format!("open chunk failed: {error}"))?;
    file.seek(SeekFrom::Start(start))
        .map_err(|error| format!("seek chunk failed: {error}"))?;
    let mut buffer = vec![0_u8; length];
    let mut read_total = 0;
    while read_total < buffer.len() {
        let read = file
            .read(&mut buffer[read_total..])
            .map_err(|error| format!("read chunk failed: {error}"))?;
        if read == 0 {
            buffer.truncate(read_total);
            break;
        }
        read_total += read;
    }
    if read_total != length {
        return Err("upload file changed while reading".into());
    }
    Ok(buffer)
}

async fn upload_file(
    app: &tauri::AppHandle,
    origin: String,
    room_id: Option<String>,
    path: PathBuf,
) -> Result<String, String> {
    let filename = path
        .file_name()
        .and_then(|value| value.to_str())
        .ok_or("invalid upload filename")?
        .to_string();
    let to_room = room_id.is_some();
    let hash_app = app.clone();
    let hash_path = path.clone();
    let hash_name = filename.clone();
    let hash_room = room_id.clone();
    let (size, sha1) = tauri::async_runtime::spawn_blocking(move || {
        hash_file(&hash_app, &hash_path, &hash_name, hash_room)
    })
    .await
    .map_err(|error| format!("hash worker failed: {error}"))??;
    let count = 1000;
    let expire = if to_room { 31536000 } else { 86400 };
    let client = network_client()?;

    let init_url = format!("{origin}/api/clip/upload/init");
    let init_response = client
        .post(&init_url)
        .json(&UploadInitRequest {
            filename: &filename,
            size,
            sha1: &sha1,
            count,
            expire,
        })
        .send()
        .await
        .map_err(|error| format!("upload init failed: {error}"))?;
    if !init_response.status().is_success() {
        return Err(format!("upload init failed ({})", init_response.status()));
    }
    let init: UploadInitResponse = init_response
        .json()
        .await
        .map_err(|error| format!("invalid upload init response: {error}"))?;

    if init.instant_upload {
        let code = init.code.ok_or("instant upload did not return a code")?;
        emit_upload(
            app,
            "complete",
            &filename,
            100,
            Some(code.clone()),
            room_id.clone(),
            None,
        );
        return Ok(code);
    }

    let upload_id = init
        .upload_id
        .ok_or("upload init did not return upload_id")?;
    if upload_id.is_empty()
        || !upload_id
            .bytes()
            .all(|c| c.is_ascii_alphanumeric() || c == b'-' || c == b'_')
    {
        return Err("invalid upload ID".into());
    }
    let chunk_size = usize::try_from(init.chunk_size.unwrap_or(HASH_CHUNK_SIZE as u64))
        .map_err(|_| "invalid server chunk size")?;
    if chunk_size == 0 || chunk_size > 64 * 1024 * 1024 {
        return Err("invalid server chunk size".to_string());
    }
    let total_chunks = init
        .total_chunks
        .unwrap_or_else(|| usize::try_from(size.div_ceil(chunk_size as u64)).unwrap_or(1));
    if total_chunks as u64 != size.div_ceil(chunk_size as u64) {
        return Err("invalid server chunk count".to_string());
    }
    let uploaded = init.uploaded_chunks.unwrap_or_default();
    let uploaded_set: std::collections::HashSet<usize> = uploaded
        .into_iter()
        .filter(|index| *index < total_chunks)
        .collect();
    let remaining: Arc<Vec<usize>> = Arc::new(
        (0..total_chunks)
            .filter(|index| !uploaded_set.contains(index))
            .collect(),
    );
    let worker_count = init
        .workers
        .unwrap_or(UPLOAD_WORKERS)
        .clamp(1, remaining.len().clamp(1, UPLOAD_WORKERS));
    let cursor = Arc::new(AtomicUsize::new(0));
    let completed = Arc::new(AtomicUsize::new(uploaded_set.len()));
    let mut handles = Vec::with_capacity(worker_count);

    for _ in 0..worker_count {
        let client = client.clone();
        let origin = origin.clone();
        let upload_id = upload_id.clone();
        let path = path.clone();
        let remaining = Arc::clone(&remaining);
        let cursor = Arc::clone(&cursor);
        let completed = Arc::clone(&completed);
        let app = app.clone();
        let filename = filename.clone();
        let total_chunks = total_chunks.max(1);
        let room_id = room_id.clone();
        handles.push(tauri::async_runtime::spawn(async move {
            loop {
                let index = cursor.fetch_add(1, Ordering::SeqCst);
                let Some(&chunk_index) = remaining.get(index) else {
                    return Ok(());
                };
                let start = u64::try_from(chunk_index)
                    .ok()
                    .and_then(|index| index.checked_mul(chunk_size as u64))
                    .ok_or("chunk offset overflow")?;
                let length = size.saturating_sub(start).min(chunk_size as u64) as usize;
                let chunk_path = path.clone();
                let bytes = tauri::async_runtime::spawn_blocking(move || {
                    read_chunk(&chunk_path, start, length)
                })
                .await
                .map_err(|error| format!("chunk reader failed: {error}"))??;
                let response = client
                    .put(format!(
                        "{origin}/api/clip/upload/{upload_id}/{chunk_index}"
                    ))
                    .header(reqwest::header::CONTENT_TYPE, "application/octet-stream")
                    .body(bytes)
                    .send()
                    .await
                    .map_err(|error| format!("upload chunk failed: {error}"))?;
                if !response.status().is_success() {
                    return Err(format!("upload chunk failed ({})", response.status()));
                }
                let finished = completed.fetch_add(1, Ordering::SeqCst) + 1;
                let progress = ((finished * 100) / total_chunks).min(100) as u8;
                emit_upload(
                    &app,
                    "uploading",
                    &filename,
                    progress,
                    None,
                    room_id.clone(),
                    None,
                );
            }
        }));
    }

    let mut worker_error = None;
    for handle in handles {
        match handle.await {
            Ok(Ok(())) => {}
            Ok(Err(error)) => {
                worker_error.get_or_insert(error);
            }
            Err(error) => {
                worker_error.get_or_insert(format!("upload worker failed: {error}"));
            }
        }
    }
    if let Some(error) = worker_error {
        return Err(error);
    }

    let complete_url = format!("{origin}/api/clip/upload/{upload_id}/complete");
    let complete_response = client
        .post(&complete_url)
        .json(&UploadCompleteRequest {
            filename: &filename,
            count,
            expire,
        })
        .send()
        .await
        .map_err(|error| format!("upload complete failed: {error}"))?;
    if !complete_response.status().is_success() {
        return Err(format!(
            "upload complete failed ({})",
            complete_response.status()
        ));
    }
    let complete: UploadCompleteResponse = complete_response
        .json()
        .await
        .map_err(|error| format!("invalid upload complete response: {error}"))?;
    let code = complete
        .code
        .ok_or("upload complete did not return a code")?;
    emit_upload(
        app,
        "complete",
        &filename,
        100,
        Some(code.clone()),
        room_id.clone(),
        None,
    );
    Ok(code)
}

fn start_floating_upload(app: tauri::AppHandle, runtime: DesktopRuntime, path: PathBuf) {
    // Capture the destination when dropped, before waiting behind another upload.
    let room_id = runtime.room_id();
    let origin = runtime.backend_origin();
    tauri::async_runtime::spawn(async move {
        let _queue = runtime.upload_queue.lock().await;
        let filename = path
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("file")
            .to_string();
        if let Err(error) = upload_file(&app, origin, room_id.clone(), path).await {
            emit_upload(&app, "error", &filename, 0, None, room_id, Some(error));
        }
    });
}

#[cfg(target_os = "macos")]
fn start_keyboard_listener(app: tauri::AppHandle, shared_text: Arc<Mutex<ClipboardState>>) {
    macos_keyboard::start(app, shared_text);
}

#[cfg(target_os = "macos")]
fn read_copied_text(app: tauri::AppHandle, shared_text: Arc<Mutex<ClipboardState>>) {
    thread::spawn(move || {
        // Wait for the foreground application to handle the forwarded copy key.
        thread::sleep(Duration::from_millis(90));
        let Ok(state) = shared_text.lock() else {
            return;
        };
        if !state.enabled {
            return;
        }
        let result = objc2::rc::autoreleasepool(|_| -> Result<(), String> {
            let mut clipboard = Clipboard::new().map_err(|error| error.to_string())?;
            if clipboard
                .get()
                .file_list()
                .is_ok_and(|files| !files.is_empty())
                || clipboard.get_image().is_ok()
            {
                return Ok(());
            }
            let text = clipboard.get_text().map_err(|error| error.to_string())?;
            if !text.is_empty() {
                app.emit("clipboard://copy", ClipboardCopyPayload { text })
                    .map_err(|error| error.to_string())?;
                eprintln!("[clipboard] copy: text emitted to room frontend");
            }
            Ok(())
        });
        if let Err(error) = result {
            eprintln!("[clipboard] copy failed: {error}");
        }
    });
}

#[cfg(not(target_os = "macos"))]
fn start_keyboard_listener(app: tauri::AppHandle, shared_text: Arc<Mutex<ClipboardState>>) {
    thread::spawn(move || {
        let keys = Arc::new(Mutex::new(KeyState::default()));
        let callback_keys = Arc::clone(&keys);
        let callback = move |event: Event| {
            let Ok(mut state) = callback_keys.lock() else {
                return;
            };
            let Ok(clipboard_state) = shared_text.lock() else {
                return;
            };
            if !clipboard_state.enabled {
                *state = KeyState::default();
                return;
            }
            let pressed = matches!(event.event_type, EventType::KeyPress(_));
            let released = matches!(event.event_type, EventType::KeyRelease(_));
            let key = match event.event_type {
                EventType::KeyPress(key) | EventType::KeyRelease(key) => key,
                _ => return,
            };
            let shortcut_modifier = if cfg!(target_os = "macos") {
                state.meta
            } else {
                state.ctrl
            };

            match key {
                Key::ControlLeft | Key::ControlRight => state.ctrl = pressed && !released,
                Key::MetaLeft | Key::MetaRight => state.meta = pressed && !released,
                Key::KeyC if released => state.c = false,
                Key::KeyV if released => state.v = false,
                Key::KeyC if pressed && !state.c && shortcut_modifier => {
                    state.c = true;
                    let copy_app = app.clone();
                    let copy_state = shared_text.clone();
                    thread::spawn(move || {
                        // Let the application receiving Ctrl/Cmd+C populate the clipboard first.
                        thread::sleep(Duration::from_millis(90));
                        let Ok(state) = copy_state.lock() else {
                            return;
                        };
                        if !state.enabled {
                            return;
                        }
                        let Ok(mut clipboard) = Clipboard::new() else {
                            return;
                        };
                        // A clipboard may expose a text fallback for an image or a copied file.
                        // Current ClipBox protocol deliberately accepts text-only clipboards.
                        if clipboard
                            .get()
                            .file_list()
                            .is_ok_and(|files| !files.is_empty())
                            || clipboard.get_image().is_ok()
                        {
                            return;
                        }
                        let Ok(text) = clipboard.get_text() else {
                            return;
                        };
                        if !text.is_empty() {
                            let _ =
                                copy_app.emit("clipboard://copy", ClipboardCopyPayload { text });
                        }
                    });
                }
                Key::KeyV if pressed && !state.v && shortcut_modifier => {
                    state.v = true;
                    let text = clipboard_state.text.clone();
                    if let Some(text) = text {
                        if let Ok(mut clipboard) = Clipboard::new() {
                            let _ = clipboard.set_text(text);
                        }
                    }
                }
                _ => {}
            }
        };
        if let Err(error) = listen(callback) {
            eprintln!("global keyboard listener stopped: {error:?}");
        }
    });
}

fn trusted_navigation(url: &reqwest::Url, dev_url: Option<&reqwest::Url>) -> bool {
    if let Some(dev) = dev_url {
        return url.origin() == dev.origin();
    }
    url.username().is_empty()
        && url.password().is_none()
        && url.host_str() == Some("tauri.localhost")
        && url.port().is_none()
        && matches!(url.scheme(), "http" | "https")
        || (url.scheme() == "tauri"
            && url.host_str() == Some("localhost")
            && url.username().is_empty()
            && url.password().is_none()
            && url.port().is_none())
}

fn create_main_window(app: &tauri::AppHandle, data_directory: &Path) -> tauri::Result<()> {
    let dev_url = if cfg!(dev) {
        app.config().build.dev_url.clone()
    } else {
        None
    };
    let navigation_dev_url = dev_url.clone();
    let builder = WebviewWindowBuilder::new(app, "main", WebviewUrl::App("index.html".into()))
        .on_navigation(move |url| trusted_navigation(url, navigation_dev_url.as_ref()))
        .on_new_window(|url, _| {
            // Open web links outside the privileged WebView; reject file/custom schemes.
            if http_url(url.as_str()).is_ok() {
                if let Err(error) = open::that_detached(url.as_str()) {
                    eprintln!("could not open external link: {error}");
                }
            }
            tauri::webview::NewWindowResponse::Deny
        })
        .title("ClipBox")
        .inner_size(1120.0, 760.0)
        .min_inner_size(720.0, 560.0)
        .center()
        .disable_drag_drop_handler()
        .data_directory(data_directory.to_path_buf());
    // WKWebView cannot redirect its persistent data directory. Use an ephemeral
    // WebView and persist application storage ourselves beside the executable.
    #[cfg(target_os = "macos")]
    let builder = {
        let storage_path = data_directory.join("local-storage.json");
        let stored: std::collections::BTreeMap<String, String> = std::fs::read(storage_path)
            .ok()
            .and_then(|bytes| serde_json::from_slice(&bytes).ok())
            .unwrap_or_default();
        let script = format!(
            "(() => {{ if (!({})) return; window.__CLIPBOX_STORAGE__ = {};\n{} }})();",
            if let Some(dev) = dev_url.as_ref() {
                format!(
                    "location.origin === {}",
                    serde_json::to_string(&dev.origin().ascii_serialization()).unwrap()
                )
            } else {
                "location.protocol === 'tauri:' && location.host === 'localhost' || ['http:', 'https:'].includes(location.protocol) && location.host === 'tauri.localhost'".into()
            },
            serde_json::to_string(&stored).unwrap(),
            include_str!("portable-storage.js")
        );
        builder.incognito(true).initialization_script(script)
    };
    builder.build()?;
    Ok(())
}

fn create_floating_window(app: &tauri::AppHandle, data_directory: &Path) -> tauri::Result<()> {
    let dev_url = if cfg!(dev) {
        app.config().build.dev_url.clone()
    } else {
        None
    };
    let builder =
        WebviewWindowBuilder::new(app, "floating", WebviewUrl::App("floating.html".into()))
            .on_navigation(move |url| trusted_navigation(url, dev_url.as_ref()))
            .on_new_window(|_, _| tauri::webview::NewWindowResponse::Deny)
            .on_menu_event(|window, event| {
                if event.id.as_ref() == "hide-floating" {
                    let _ = window.hide();
                }
            })
            .title("ClipBox Floating Ball")
            .inner_size(48.0, 48.0)
            .resizable(false)
            .decorations(false)
            .transparent(true)
            .shadow(false)
            .always_on_top(true)
            .skip_taskbar(true)
            .position(48.0, 96.0)
            .data_directory(data_directory.to_path_buf());
    // Match the main window's portable WKWebView storage setup on macOS.
    #[cfg(target_os = "macos")]
    let builder = builder.incognito(true);
    builder.build()?;
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // Set process and child-process temporary paths before any threads/WebViews start.
    let temp_directory =
        create_dir_under_executable("data/tmp").expect("portable directory is not writable");
    for variable in ["TEMP", "TMP", "TMPDIR"] {
        std::env::set_var(variable, &temp_directory);
    }
    std::env::set_current_dir(executable_directory())
        .expect("could not set portable working directory");
    tauri::Builder::default()
        .manage(DesktopRuntime::default())
        .invoke_handler(tauri::generate_handler![
            set_shared_text,
            set_clipboard_sharing,
            show_main_window,
            show_floating_menu,
            set_backend_origin,
            set_upload_target,
            save_room_file,
            save_portable_storage
        ])
        .setup(|app| {
            // Portable macOS executables have no .app bundle to supply a Dock icon.
            #[cfg(target_os = "macos")]
            {
                use objc2::{AllocAnyThread, MainThreadMarker};
                use objc2_app_kit::{NSApplication, NSImage};
                use objc2_foundation::NSData;
                let mtm = MainThreadMarker::new().ok_or("setup must run on the main thread")?;
                let data = NSData::with_bytes(include_bytes!("../icons/icon.png"));
                if let Some(icon) = NSImage::initWithData(NSImage::alloc(), &data) {
                    unsafe {
                        NSApplication::sharedApplication(mtm).setApplicationIconImage(Some(&icon))
                    };
                }
            }
            let data_directory =
                create_dir_under_executable("data").map_err(std::io::Error::other)?;
            create_main_window(app.handle(), &data_directory)?;
            create_floating_window(app.handle(), &data_directory)?;

            let show = MenuItem::with_id(app, "show", "打开 ClipBox", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let floating =
                MenuItem::with_id(app, "toggle-floating", "显示悬浮窗", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;
            menu.append(&floating)?;

            let mut tray = TrayIconBuilder::new()
                .menu(&menu)
                .tooltip("ClipBox")
                .show_menu_on_left_click(false)
                .on_menu_event({
                    move |app, event| match event.id.as_ref() {
                        "show" => {
                            if let Some(window) = app.get_webview_window("main") {
                                let _ = window.show();
                                let _ = window.set_focus();
                            }
                        }
                        "quit" => app.exit(0),
                        "toggle-floating" => {
                            if let Some(window) = app.get_webview_window("floating") {
                                let _ = window.show();
                            }
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        if let Some(window) = tray.app_handle().get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                });
            if let Some(icon) = app.default_window_icon() {
                tray = tray.icon(icon.clone());
            }
            tray.build(app)?;

            let runtime = app.state::<DesktopRuntime>().inner().clone();
            start_keyboard_listener(app.handle().clone(), runtime.shared_text.clone());
            Ok(())
        })
        .on_window_event(move |window, event| {
            let runtime = window
                .app_handle()
                .state::<DesktopRuntime>()
                .inner()
                .clone();
            if window.label() == "floating" {
                match event {
                    WindowEvent::DragDrop(DragDropEvent::Enter { .. }) => {
                        let _ = window
                            .app_handle()
                            .emit("floating://drag", FloatingDragPayload { dragging: true });
                    }
                    WindowEvent::DragDrop(DragDropEvent::Leave) => {
                        let _ = window
                            .app_handle()
                            .emit("floating://drag", FloatingDragPayload { dragging: false });
                    }
                    WindowEvent::DragDrop(DragDropEvent::Drop { paths, .. }) => {
                        let app = window.app_handle().clone();
                        let _ =
                            app.emit("floating://drag", FloatingDragPayload { dragging: false });
                        for path in paths {
                            start_floating_upload(app.clone(), runtime.clone(), path.clone());
                        }
                    }
                    _ => {}
                }
            }

            if let WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running ClipBox desktop client");
}

#[cfg(test)]
mod tests {
    #[test]
    fn disabling_clipboard_sharing_discards_and_rejects_room_text() {
        let mut state = super::ClipboardState::default();
        state.cache_text(Some("before joining".into()));
        assert!(state.text.is_none());
        state.set_enabled(true);
        state.cache_text(Some("room text".into()));
        assert_eq!(state.text.as_deref(), Some("room text"));
        state.set_enabled(false);
        assert!(state.text.is_none());
        state.cache_text(Some("received while disabled".into()));
        assert!(state.text.is_none());
        state.set_enabled(true);
        assert!(state.text.is_none());
        state.cache_text(Some("latest room text".into()));
        assert_eq!(state.text.as_deref(), Some("latest room text"));
    }

    use super::*;

    #[test]
    fn network_client_does_not_follow_redirects() {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let server = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let mut request = [0; 4096];
            stream.read(&mut request).unwrap();
            stream.write_all(b"HTTP/1.1 302 Found\r\nLocation: http://127.0.0.1:1/private\r\nContent-Length: 0\r\nConnection: close\r\n\r\n").unwrap();
        });
        tauri::async_runtime::block_on(async {
            let response = network_client()
                .unwrap()
                .get(format!("http://{address}/"))
                .bearer_auth("test-token")
                .send()
                .await
                .unwrap();
            assert_eq!(response.status(), reqwest::StatusCode::FOUND);
        });
        server.join().unwrap();
    }

    #[test]
    fn downloads_reject_unregistered_origins_and_non_room_paths() {
        let origins = [
            "https://box.example:444".to_string(),
            "http://127.0.0.1:5328".to_string(),
        ]
        .into_iter()
        .collect();
        for url in [
            "https://box.example:444/api/rooms/12345/messages/42/file",
            "http://127.0.0.1:5328/api/rooms/12345/messages/42/file",
        ] {
            assert!(validate_download_url(url, &origins).is_ok());
        }
        for url in [
            "https://evil.example/api/rooms/12345/messages/42/file",
            "https://box.example:444/admin",
            "https://user:pass@box.example:444/api/rooms/12345/messages/42/file",
            "file:///etc/passwd",
            "https://box.example:444/api/rooms/12345/messages/42/file?redirect=evil",
        ] {
            assert!(validate_download_url(url, &origins).is_err(), "{url}");
        }
    }

    #[test]
    fn navigation_rejects_remote_pages_and_confusable_hosts() {
        for value in [
            "tauri://localhost/index.html",
            "http://tauri.localhost/",
            "https://tauri.localhost/rooms",
        ] {
            assert!(trusted_navigation(&value.parse().unwrap(), None));
        }
        for value in [
            "https://evil.example",
            "http://tauri.localhost.evil.example",
            "tauri://evil/",
            "http://user@tauri.localhost",
            "http://tauri.localhost:8080",
            "file:///etc/passwd",
        ] {
            assert!(
                !trusted_navigation(&value.parse().unwrap(), None),
                "{value}"
            );
        }
        let dev = "http://localhost:5173".parse().unwrap();
        assert!(trusted_navigation(
            &"http://localhost:5173/rooms".parse().unwrap(),
            Some(&dev)
        ));
        assert!(!trusted_navigation(
            &"http://localhost:5174/".parse().unwrap(),
            Some(&dev)
        ));
    }

    #[test]
    fn filenames_are_safe_on_windows() {
        assert_eq!(safe_filename("../report.txt"), "report.txt");
        assert_eq!(safe_filename("NUL.txt"), "_NUL.txt");
        assert_eq!(safe_filename("com1"), "_com1");
        assert_eq!(safe_filename("bad:na\u{0}me?.txt. "), "bad_na_me_.txt");
        assert_eq!(safe_filename("..."), "download");
    }

    #[test]
    fn simultaneous_same_name_downloads_never_overwrite() {
        let directory = std::env::temp_dir().join(format!(
            "clipbox-test-{}-{}",
            std::process::id(),
            chrono::Utc::now().timestamp_nanos_opt().unwrap()
        ));
        create_dir_all(&directory).unwrap();
        let handles: Vec<_> = (0..12)
            .map(|index| {
                let directory = directory.clone();
                thread::spawn(move || {
                    let (path, mut file) =
                        create_unique_download_file(&directory, "report.txt").unwrap();
                    let bytes = format!("contents {index}");
                    file.write_all(bytes.as_bytes()).unwrap();
                    (path, bytes)
                })
            })
            .collect();
        let mut paths = std::collections::HashSet::new();
        for handle in handles {
            let (path, bytes) = handle.join().unwrap();
            assert_eq!(std::fs::read_to_string(&path).unwrap(), bytes);
            assert!(paths.insert(path));
        }
        assert!(paths.contains(&directory.join("report.txt")));
        for path in &paths {
            assert_eq!(path.extension().unwrap(), "txt");
            assert!(path
                .file_name()
                .unwrap()
                .to_string_lossy()
                .starts_with("report"));
        }
        std::fs::remove_dir_all(directory).unwrap();
    }
}
