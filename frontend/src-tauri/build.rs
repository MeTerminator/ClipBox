fn main() {
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            "set_shared_text",
            "set_backend_origin",
            "set_upload_target",
            "save_room_file",
            "save_portable_storage",
        ]),
    ))
    .expect("failed to build Tauri context");
}
