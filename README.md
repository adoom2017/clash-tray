# Clash Tray · Rust

轻量 Windows 托盘程序，管理外部 `mihomo.exe`，不包含代理内核或配置编辑器。

## 编译

需要 Windows 10/11、Rust stable（支持 edition 2024）、Visual Studio C++ Build Tools 和 Windows SDK。

```powershell
cargo build --release --locked
cargo test --locked
cargo clippy --all-targets --locked -- -D warnings
```

产物为 `target/release/ClashTray.exe`。无需 Go 或 WebView。

## 使用

将程序放到可写目录：

```text
ClashTray.exe
mihomo.exe
config/
  config.yaml
```

双击后右键托盘图标，可启动/停止 Clash、显示日志、更新 Mihomo、开机自启动和退出。启动托盘不会自动启动内核，与原版一致。没有内核时可通过更新菜单首次下载，配置需自行准备。

配置与日志均相对程序目录。日志同时捕获 stdout/stderr，超过 10 MiB 轮转为 `mihomo.log.1`，保留一个备份。

默认普通用户权限，方便 HKCU Run 自启动。TUN 模式需要权限时，请右键“以管理员身份运行”。这与原 Go 版强制 UAC 提权不同，自启动不会自动提权。

更新从 GitHub 官方 release 下载，按当前架构选择 ZIP，先完整解压再停止内核、备份并替换；替换失败恢复旧文件，之前运行的内核会重启。下载时禁用启停操作，可以退出。支持 x64 compatible、ARM64、x86 资源名，本地构建验证仅覆盖 x64。

## 结构

- `src/main.rs`：托盘和 Win32 消息循环，主线程集中管理状态。
- `src/process.rs`：子进程和日志轮转。
- `src/update.rs`：下载、解压和替换回滚。
- `src/autostart.rs`：当前用户注册表自启动。
- `legacy/go/`：迁移前 Go 源码，包含原有未提交修改。
- `MIGRATION.md`：迁移分析和验证范围。

托盘使用 [tray-icon](https://docs.rs/tray-icon/0.24.2/tray_icon/)，图标资源使用 [winresource](https://docs.rs/winresource/0.1.31/winresource/)。

当前版本与 GitHub 最新版本相同时跳过下载。
