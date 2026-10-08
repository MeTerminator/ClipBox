//! macOS shortcut tap: no input-source translation and synchronous paste handling.
use arboard::Clipboard;
use core_foundation::{
    base::TCFType, boolean::CFBoolean, dictionary::CFDictionary, string::CFString,
};
use core_graphics::event::{CGEvent, EventField};
use foreign_types::ForeignType;
use std::{
    ffi::c_void,
    mem::ManuallyDrop,
    ptr,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

type Ref = *const c_void;
type EventRef = *mut c_void;
const KEY_DOWN: u32 = 10;
const COMMAND: u64 = 1 << 20;
const TAP_DISABLED_TIMEOUT: u32 = 0xfffffffe;
const TAP_DISABLED_INPUT: u32 = 0xffffffff;

#[link(name = "ApplicationServices", kind = "framework")]
extern "C" {
    fn AXIsProcessTrusted() -> u8;
    fn AXIsProcessTrustedWithOptions(options: Ref) -> u8;
    fn CGEventTapCreate(
        location: u32,
        placement: u32,
        options: u32,
        mask: u64,
        callback: unsafe extern "C" fn(EventRef, u32, EventRef, *mut c_void) -> EventRef,
        user_info: *mut c_void,
    ) -> Ref;
    fn CGEventTapEnable(tap: Ref, enable: bool);
}
#[link(name = "CoreFoundation", kind = "framework")]
extern "C" {
    fn CFMachPortCreateRunLoopSource(allocator: Ref, tap: Ref, order: isize) -> Ref;
    fn CFRunLoopGetCurrent() -> Ref;
    fn CFRunLoopAddSource(run_loop: Ref, source: Ref, mode: Ref);
    fn CFRunLoopRemoveSource(run_loop: Ref, source: Ref, mode: Ref);
    fn CFRunLoopRun();
    fn CFMachPortInvalidate(tap: Ref);
    fn CFRelease(value: Ref);
    static kCFRunLoopCommonModes: Ref;
}

#[derive(Debug, PartialEq)]
enum Shortcut {
    Copy,
    Paste,
}

fn shortcut(event_type: u32, key: i64, flags: u64, repeat: i64) -> Option<Shortcut> {
    // Read Command from each event, rather than relying on missed modifier events
    // or comparing the numeric value of unrelated modifier flags.
    if event_type != KEY_DOWN || flags & COMMAND == 0 || repeat != 0 {
        return None;
    }
    match key {
        8 => Some(Shortcut::Copy),
        9 => Some(Shortcut::Paste),
        _ => None,
    }
}

struct Context {
    app: tauri::AppHandle,
    text: Arc<Mutex<Option<String>>>,
    tap: Ref,
}

unsafe extern "C" fn callback(
    _proxy: EventRef,
    event_type: u32,
    event: EventRef,
    data: *mut c_void,
) -> EventRef {
    if data.is_null() {
        return event;
    }
    // Only this run-loop thread accesses the context; it outlives the tap.
    let context = &*(data as *const Context);
    if event_type == TAP_DISABLED_TIMEOUT || event_type == TAP_DISABLED_INPUT {
        CGEventTapEnable(context.tap, true);
        eprintln!("[clipboard] keyboard tap re-enabled after macOS disabled it");
        return event;
    }
    if event.is_null() {
        return event;
    }
    // Borrow the OS-owned event without releasing it or crossing the C ABI
    // with a Rust wrapper. Return the original event to preserve normal keys.
    let event_ref = ManuallyDrop::new(CGEvent::from_ptr(event.cast()));
    let action = shortcut(
        event_type,
        event_ref.get_integer_value_field(EventField::KEYBOARD_EVENT_KEYCODE),
        event_ref.get_flags().bits(),
        event_ref.get_integer_value_field(EventField::KEYBOARD_EVENT_AUTOREPEAT),
    );
    // Never unwind through the C callback boundary.
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        objc2::rc::autoreleasepool(|_| match action {
            Some(Shortcut::Copy) => {
                eprintln!("[clipboard] Command+C received");
                super::read_copied_text(context.app.clone());
            }
            Some(Shortcut::Paste) => {
                let text = context.text.lock().ok().and_then(|value| value.clone());
                if let Some(text) = text {
                    // An active Session tap holds delivery until this returns, so
                    // the receiving application sees the new clipboard on Cmd+V.
                    match Clipboard::new().and_then(|mut clipboard| clipboard.set_text(text)) {
                        Ok(()) => {
                            eprintln!(
                                "[clipboard] Command+V: room text written before key delivery"
                            )
                        }
                        Err(error) => eprintln!("[clipboard] paste failed: {error}"),
                    }
                } else {
                    eprintln!("[clipboard] Command+V: no room text cached");
                }
            }
            None => {}
        })
    }));
    if result.is_err() {
        eprintln!("[clipboard] shortcut handler panicked; key forwarded");
    }
    event
}

pub fn start(app: tauri::AppHandle, text: Arc<Mutex<Option<String>>>) {
    // Called from Tauri setup on the main thread. Prompt once; granting is async.
    let options = CFDictionary::from_CFType_pairs(&[(
        CFString::new("AXTrustedCheckOptionPrompt"),
        CFBoolean::true_value(),
    )]);
    let trusted =
        unsafe { AXIsProcessTrustedWithOptions(options.as_concrete_TypeRef().cast()) != 0 };
    eprintln!(
        "[clipboard] accessibility permission: {}",
        if trusted {
            "granted"
        } else {
            "missing; grant ClipBox or its launching terminal in System Settings"
        }
    );
    thread::spawn(move || {
        let mut waiting = false;
        loop {
            if unsafe { AXIsProcessTrusted() } == 0 {
                if !waiting {
                    eprintln!("[clipboard] waiting for Accessibility permission; will retry automatically");
                    waiting = true;
                }
                thread::sleep(Duration::from_secs(2));
                continue;
            }
            waiting = false;
            let mut context = Box::new(Context {
                app: app.clone(),
                text: Arc::clone(&text),
                tap: ptr::null(),
            });
            unsafe {
                // Session (1), HeadInsert (0), active Default (0): no HID/root
                // requirement and paste is processed before the key is forwarded.
                let tap = CGEventTapCreate(
                    1,
                    0,
                    0,
                    1 << KEY_DOWN,
                    callback,
                    (&mut *context as *mut Context).cast(),
                );
                if tap.is_null() {
                    eprintln!("[clipboard] keyboard tap creation failed; check Accessibility permission for this executable; retrying in 5s");
                    thread::sleep(Duration::from_secs(5));
                    continue;
                }
                context.tap = tap;
                let source = CFMachPortCreateRunLoopSource(ptr::null(), tap, 0);
                if source.is_null() {
                    CFMachPortInvalidate(tap);
                    CFRelease(tap);
                    eprintln!("[clipboard] failed to create keyboard run-loop source; retrying");
                    thread::sleep(Duration::from_secs(5));
                    continue;
                }
                let run_loop = CFRunLoopGetCurrent();
                CFRunLoopAddSource(run_loop, source, kCFRunLoopCommonModes);
                CGEventTapEnable(tap, true);
                eprintln!("[clipboard] macOS keyboard listener ready (Session tap)");
                CFRunLoopRun();
                CGEventTapEnable(tap, false);
                CFRunLoopRemoveSource(run_loop, source, kCFRunLoopCommonModes);
                CFMachPortInvalidate(tap);
                CFRelease(source);
                CFRelease(tap);
            }
            eprintln!("[clipboard] keyboard run loop ended; restarting");
            thread::sleep(Duration::from_secs(2));
        }
    });
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn shortcuts_use_current_command_flag_without_modifier_history() {
        assert_eq!(shortcut(KEY_DOWN, 8, COMMAND, 0), Some(Shortcut::Copy));
        assert_eq!(
            shortcut(KEY_DOWN, 9, COMMAND | (1 << 17), 0),
            Some(Shortcut::Paste)
        );
        assert_eq!(shortcut(KEY_DOWN, 8, 1 << 18, 0), None);
        assert_eq!(shortcut(KEY_DOWN, 9, 0, 0), None);
        assert_eq!(shortcut(KEY_DOWN, 9, COMMAND, 1), None);
        assert_eq!(shortcut(11, 9, COMMAND, 0), None);
    }
}
