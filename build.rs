fn main() {
    println!("cargo:rerun-if-changed=app-disable.ico");
    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("windows") {
        winresource::WindowsResource::new()
            .set_icon("app-disable.ico")
            .set("ProductName", "Clash Tray")
            .compile()
            .expect("Windows resource compilation failed");
    }
}
