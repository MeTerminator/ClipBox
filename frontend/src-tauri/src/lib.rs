use std::{
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use arboard::Clipboard;
use rdev::{listen, Event, EventType, Key};
use serde::Serialize;
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Emitter, Manager, State, WindowEvent,
};

#[derive(Default)]
struct ClipboardState(Arc<Mutex<Option<String>>>);

#[derive(Clone, Serialize)]
struct ClipboardCopyPayload {
    text: String,
}

#[derive(Default)]
struct KeyState {
    ctrl: bool,
    meta: bool,
    c: bool,
    v: bool,
}

#[tauri::command]
fn set_shared_text(state: State<'_, ClipboardState>, text: Option<String>) {
    if let Ok(mut current) = state.0.lock() {
        *current = text.filter(|value| !value.is_empty());
    }
}

fn start_keyboard_listener(app: tauri::AppHandle, shared_text: Arc<Mutex<Option<String>>>) {
    thread::spawn(move || {
        let keys = Arc::new(Mutex::new(KeyState::default()));
        let callback_keys = Arc::clone(&keys);
        let callback = move |event: Event| {
            let Ok(mut state) = callback_keys.lock() else {
                return;
            };
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
                    thread::spawn(move || {
                        // Let the application receiving Ctrl/Cmd+C populate the clipboard first.
                        thread::sleep(Duration::from_millis(90));
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
                    let text = shared_text.lock().ok().and_then(|value| value.clone());
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

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(ClipboardState::default())
        .invoke_handler(tauri::generate_handler![set_shared_text])
        .setup(|app| {
            let show = MenuItem::with_id(app, "show", "打开 ClipBox", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &quit])?;
            let mut tray = TrayIconBuilder::new()
                .menu(&menu)
                .tooltip("ClipBox")
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "show" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                    "quit" => app.exit(0),
                    _ => {}
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

            let shared_text = app.state::<ClipboardState>().0.clone();
            start_keyboard_listener(app.handle().clone(), shared_text);
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running ClipBox desktop client");
}
