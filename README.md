# Clash Tray

<p>
  <img src="assets/icons/running.png" width="96" alt="运行状态：蓝青色">
  <img src="assets/icons/stopped.png" width="96" alt="停止状态：石墨灰">
</p>

使用 Rust 编写的轻量 Windows 托盘工具，用于管理 Mihomo 内核。启动后自动运行 Mihomo，通过原生托盘菜单操作，不显示主窗口或内核控制台。

项目不内置 Mihomo、订阅管理或配置编辑器。代理规则、端口和 TUN 设置由 Mihomo 配置文件决定，托盘不会修改 Windows 系统代理设置。

## 功能

- 启动时请求管理员权限，并自动启动 Mihomo，支持创建 TUN 接口。
- 手动启动、停止内核，使用不同颜色的托盘图标显示进程运行状态。
- 捕获 stdout / stderr，支持查看日志与日志轮转。
- 检查并下载 Mihomo 更新，替换前备份，失败时尝试恢复。
- 使用最高权限计划任务，在当前用户登录后自动启动。
- 退出托盘时停止由本程序启动的内核。

## 下载与部署

发布版本可在 [GitHub Releases](https://github.com/adoom2017/clash-tray/releases) 查看。Rust 版发布包命名为 `ClashTray-v版本号-windows-x64.zip`，附带 `.zip.sha256` 校验文件；包内不包含内核和用户配置。

将程序解压到可写目录，准备以下文件：

```text
ClashTray/
├── ClashTray.exe
├── mihomo.exe
└── config/
    └── config.yaml
```

Mihomo 所需的其他数据文件也放在 `config/` 中。配置、日志和内核路径始终相对于 `ClashTray.exe` 所在目录，不依赖启动时的工作目录。

1. 双击 `ClashTray.exe`，同意 Windows UAC 管理员授权。
2. 程序显示托盘图标并自动启动 Mihomo。
3. 右键托盘可查看日志、手动启停、更新内核和设置自启动。
4. 选择“退出”会关闭托盘并停止内核。

取消 UAC 授权时程序不会启动。首次使用尚无内核时，关闭启动错误提示，在托盘选择“更新 Mihomo”；准备好 `config/config.yaml` 后，再选择“启动 Clash”。

### 日志与运行状态

“显示日志”使用记事本打开 `mihomo.log`。文件超过 10 MiB 后，在后续写入时轮转为 `mihomo.log.1`，保留一个备份。

图标显示的是内核进程状态，不代表网络连通性。配置错误、端口冲突或 TUN 初始化失败时，请查看日志中的 Mihomo 输出。

### 内核更新

“更新 Mihomo”从 GitHub Release 查询最新版本；版本相同时跳过下载。需要更新时，先下载并解压，再停止内核、备份并替换。原内核保留为 `mihomo.exe.bak`，替换失败时尝试恢复。更新前正在运行的内核会尝试重新启动。

下载期间已有内核继续运行，手动启停暂不可用，仍可退出托盘。更新需要能够访问 GitHub API 和 Release 下载地址。

资源匹配支持 x64 compatible、ARM64 和 x86；当前自动构建与本地验证覆盖 Windows x64。

### 管理员权限与登录自启动

EXE 内嵌 `requireAdministrator` 清单，普通启动时按系统 UAC 策略请求权限。Mihomo 继承托盘的管理员权限；从已提权的环境启动通常不会再次弹框。

勾选“开机自启动”后，程序创建计划任务 `ClashTray-用户SID`：

- 仅在对应用户登录后触发，延迟 10 秒启动。
- 使用最高权限，在用户交互桌面显示托盘，不保存用户密码。
- 不限制运行时长，电池供电时也可运行。
- 注册后读取任务配置，确认账户、权限、路径和触发器符合要求。

取消勾选会删除任务并清理旧版 HKCU Run 的 `ClashTray` 项，不会停止当前代理。发现旧注册表项指向当前程序时，会先创建计划任务，再删除旧项。

移动程序后，请从新位置启动并重新设置自启动。此功能面向当前登录的管理员账户；若使用其他管理员账户的凭据提权，任务归属该管理员账户。它是登录后启动的托盘程序，不是无人登录时运行的系统服务。

## 编译与开发

需要 Windows 10 / 11、Rust 2024 edition 工具链，以及包含 MSVC 和 Windows SDK 的 Visual Studio C++ Build Tools。CI 使用 Rust **1.93.1** 和 Windows x64。首次构建需要联网下载 Cargo 依赖。

在项目根目录执行：

```powershell
cargo build --release --locked
```

产物为 `target/release/ClashTray.exe`。开发模式可使用 `cargo run --locked`，但应在**管理员 PowerShell** 中运行，否则可能返回“请求的操作需要提升”（错误 740）。开发模式的内核、配置和日志应放在 `target/debug/` 下。

### 检查与测试

```powershell
cargo fmt --check
cargo check --locked
cargo clippy --all-targets --locked -- -D warnings

# 校验账户名称/SID、空参数、路径、权限等兼容性；不修改计划任务
./scripts/test-autostart-validation.ps1

# 以下两项请在管理员 PowerShell 中执行
cargo test --locked
./scripts/test-autostart.ps1
```

Rust 测试 EXE 同样带管理员清单。`test-autostart.ps1` 创建独立临时任务，验证注册、读取、禁用和删除，结束后清理；它不会运行代理或修改正式自启动任务。真实登录启动和 TUN 网络功能仍需在实际环境中验收。

安装 Make 后，也可使用 `make build`、`make run`、`make test` 和 `make check`，权限要求与对应 Cargo 命令一致。

### 本地打包

```powershell
cargo build --release --locked
./scripts/package.ps1
```

`dist/` 中生成 ZIP 和 SHA-256 文件，包含 EXE、README、许可证及 README 使用的图标。不会打包内核、配置或日志。

## GitHub Actions 编译与发布

工作流位于 `.github/workflows/build-release.yml`：

| 触发方式 | 执行内容 |
| --- | --- |
| 分支推送、PR、手动运行 | 格式检查、Clippy、Rust 测试、自启动验证、Release 编译与打包 |
| 推送 `v*` 标签 | 完成上述检查后创建 GitHub Release，上传 ZIP 和 SHA-256 |

普通构建的产物在 Actions Artifacts 中保留 14 天。手动运行入口通常要求工作流已存在于默认分支。工作流使用自动提供的 `GITHUB_TOKEN`，只有发布任务申请 `contents: write`；无需个人访问令牌。

发布前更新 `Cargo.toml` 的版本并同步 `Cargo.lock`、`app.manifest` 中的版本信息，提交到准备发布的分支。标签必须与 Cargo 版本严格一致。以 `2.0.0` 为例：

```powershell
git push origin master
git tag v2.0.0
git push origin v2.0.0
```

请使用实际版本号，不要重复使用已有发布标签。包含 `-` 的标签会发布为预发布版本。首次启用需确认仓库或组织允许运行 GitHub Actions。

## 项目结构

```text
src/
├── main.rs                     # 托盘、Win32 消息循环及应用状态
├── process.rs                  # 内核进程、输出捕获与日志轮转
├── update.rs                   # 版本检查、下载解压和备份恢复
├── autostart.rs                # 脚本调用与旧自启动设置迁移
└── autostart.ps1               # Task Scheduler COM 操作及校验
scripts/
├── package.ps1                # 发布包与校验文件
├── test-autostart.ps1          # 管理员计划任务集成验证
└── test-autostart-validation.ps1 # 无副作用的配置校验回归测试
.github/workflows/build-release.yml # 自动构建与发布
assets/icons/                   # 图标 PNG 原图及设计说明
examples/prepare_icons.rs        # PNG 转多尺寸 ICO 工具
app-enable.ico / app-disable.ico # 运行和停止图标
app.manifest                    # 管理员权限清单
build.rs                        # Windows 图标与清单资源编译
Cargo.toml / Cargo.lock          # Rust 包配置与锁定依赖
```

主要依赖包括 `tray-icon`、`windows-sys`、`winreg`、`reqwest`、`serde`、`zip`、`tempfile`、`image` 和 `winresource`。

ICO 包含 16、20、24、32、40、48、64、128、256 像素版本。更新原图后，在管理员 PowerShell 中重新生成：

```powershell
cargo run --locked --example prepare_icons -- assets/icons/running.png app-enable.ico
cargo run --locked --example prepare_icons -- assets/icons/stopped.png app-disable.ico
```

## 许可证

Apache-2.0，详见随附的 LICENSE 文件。
