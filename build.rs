fn main() {
    println!("cargo:rerun-if-changed=app-enable.ico");
    println!("cargo:rerun-if-changed=app.manifest");
    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("windows") {
        winresource::WindowsResource::new()
            .set_icon("app-enable.ico")
            .set_manifest_file("app.manifest")
            .set("ProductName", "Clash Tray")
            .compile()
            .expect("Windows resource compilation failed");
    }
}
