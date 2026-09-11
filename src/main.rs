#![windows_subsystem = "windows"]
mod autostart;
mod process;
mod update;

use anyhow::{Context, Result};
use process::{Core, Logger};
use std::{ptr::null_mut, sync::mpsc, time::Duration};
use tray_icon::{
    Icon, TrayIconBuilder,
    menu::{CheckMenuItem, Menu, MenuEvent, MenuItem, PredefinedMenuItem},
};
use windows_sys::Win32::UI::WindowsAndMessaging::*;

fn message(text: &str, error: bool) {
    let text: Vec<u16> = text.encode_utf16().chain(Some(0)).collect();
    let title: Vec<u16> = "Clash Tray".encode_utf16().chain(Some(0)).collect();
    // Both strings remain alive and NUL-terminated for this synchronous call.
    unsafe {
        MessageBoxW(
            null_mut(),
            text.as_ptr(),
            title.as_ptr(),
            if error {
                MB_ICONERROR
            } else {
                MB_ICONINFORMATION
            },
        );
    }
}
fn icon(bytes: &[u8]) -> Result<Icon> {
    // The original ICO contains RGB PNG entries; image's ICO decoder requires RGBA.
    // Decode embedded PNGs directly so RGB entries are converted correctly.
    let mut png = None;
    if bytes.get(..4) == Some(&[0, 0, 1, 0]) && bytes.len() >= 6 {
        let count = u16::from_le_bytes([bytes[4], bytes[5]]) as usize;
        for i in 0..count {
            let Some(entry) = bytes.get(6 + i * 16..6 + (i + 1) * 16) else {
                break;
            };
            let size = u32::from_le_bytes(entry[8..12].try_into()?) as usize;
            let offset = u32::from_le_bytes(entry[12..16].try_into()?) as usize;
            if let Some(data) = offset
                .checked_add(size)
                .and_then(|end| bytes.get(offset..end))
                && data.starts_with(b"\x89PNG\r\n\x1a\n")
            {
                png = Some(image::load_from_memory_with_format(
                    data,
                    image::ImageFormat::Png,
                )?);
                break;
            }
        }
    }
    let image = match png {
        Some(image) => image,
        None => image::load_from_memory_with_format(bytes, image::ImageFormat::Ico)?,
    }
    .into_rgba8();
    let (w, h) = image.dimensions();
    Ok(Icon::from_rgba(image.into_raw(), w, h)?)
}

fn main() {
    if let Err(error) = run() {
        message(&format!("{error:#}"), true);
    }
}
fn run() -> Result<()> {
    let base = std::env::current_exe()?
        .parent()
        .context("程序目录不存在")?
        .to_path_buf();
    let log = Logger::new(base.join("mihomo.log"))?;
    let menu = Menu::new();
    let show = MenuItem::new("显示日志", true, None);
    let start = MenuItem::new("启动 Clash", true, None);
    let stop = MenuItem::new("停止 Clash", false, None);
    let update = MenuItem::new("更新 Mihomo", true, None);
    let auto = CheckMenuItem::new(
        "开机自启动",
        true,
        autostart::enabled().unwrap_or(false),
        None,
    );
    let quit = MenuItem::new("退出", true, None);
    menu.append_items(&[
        &show,
        &start,
        &stop,
        &PredefinedMenuItem::separator(),
        &update,
        &auto,
        &PredefinedMenuItem::separator(),
        &quit,
    ])?;
    let disabled = icon(include_bytes!("../app-disable.ico"))?;
    let enabled = icon(include_bytes!("../app-enable.ico"))?;
    let tray = TrayIconBuilder::new()
        .with_menu(Box::new(menu))
        .with_tooltip("Clash Tray")
        .with_icon(disabled.clone())
        .build()?;
    let mut core = Core::default();
    let (tx, rx) = mpsc::channel();
    let mut updating = false;
    let mut displayed_running = false;
    loop {
        // Pump the Win32 queue on the same thread that owns the tray and menus.
        unsafe {
            let mut msg = std::mem::zeroed();
            while PeekMessageW(&mut msg, null_mut(), 0, 0, PM_REMOVE) != 0 {
                if msg.message == WM_QUIT {
                    return Ok(());
                }
                TranslateMessage(&msg);
                DispatchMessageW(&msg);
            }
        }
        core.poll(&log)?;
        while let Ok(event) = MenuEvent::receiver().try_recv() {
            let result = if event.id == *quit.id() {
                core.stop()?;
                return Ok(());
            } else if event.id == *show.id() {
                std::process::Command::new("notepad.exe")
                    .arg(base.join("mihomo.log"))
                    .spawn()
                    .map(|_| ())
                    .map_err(Into::into)
            } else if event.id == *start.id() && !updating {
                core.start(&base, &log)
            } else if event.id == *stop.id() && !updating {
                core.stop()
            } else if event.id == *auto.id() {
                let result = autostart::set(auto.is_checked());
                auto.set_checked(autostart::enabled().unwrap_or(false));
                result
            } else if event.id == *update.id() && !updating {
                updating = true;
                update.set_text("正在下载更新...");
                update.set_enabled(false);
                let tx = tx.clone();
                let base = base.clone();
                std::thread::spawn(move || {
                    let _ = tx.send(update::prepare(&base));
                });
                Ok(())
            } else {
                Ok(())
            };
            if let Err(error) = result {
                log.write(format!("{error:#}\n").as_bytes());
                message(&format!("{error:#}"), true);
            }
        }
        if let Ok(result) = rx.try_recv() {
            let result = result.and_then(|prepared| {
                if prepared.unchanged {
                    return Ok(format!("{}（已是最新版本）", prepared.version));
                }
                let restart = core.running();
                core.stop()?;
                let installed = update::install(&prepared, &base);
                let restarted = if restart {
                    core.start(&base, &log)
                } else {
                    Ok(())
                };
                match (installed, restarted) {
                    (Err(a), Err(b)) => anyhow::bail!("{a:#}; 重启失败: {b:#}"),
                    (Err(e), _) | (_, Err(e)) => Err(e),
                    _ => Ok(prepared.version),
                }
            });
            updating = false;
            update.set_text("更新 Mihomo");
            update.set_enabled(true);
            match result {
                Ok(version) => message(&format!("Mihomo 已更新到 {version}"), false),
                Err(error) => {
                    log.write(format!("{error:#}\n").as_bytes());
                    message(&format!("更新失败: {error:#}"), true);
                }
            }
        }
        let running = core.running();
        start.set_enabled(!running && !updating);
        stop.set_enabled(running && !updating);
        if running != displayed_running {
            tray.set_icon(Some(if running {
                enabled.clone()
            } else {
                disabled.clone()
            }))?;
            displayed_running = running;
        }
        std::thread::sleep(Duration::from_millis(30));
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn embedded_icons_decode() {
        super::icon(include_bytes!("../app-disable.ico")).unwrap();
        super::icon(include_bytes!("../app-enable.ico")).unwrap();
    }
}
