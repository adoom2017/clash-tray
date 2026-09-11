use anyhow::Result;
use winreg::{RegKey, enums::*};
const KEY: &str = r"Software\Microsoft\Windows\CurrentVersion\Run";

pub fn enabled() -> Result<bool> {
    let key = RegKey::predef(HKEY_CURRENT_USER).open_subkey(KEY)?;
    let value: String = key.get_value("ClashTray").unwrap_or_default();
    Ok(value
        .trim_matches('"')
        .eq_ignore_ascii_case(&std::env::current_exe()?.to_string_lossy()))
}

pub fn set(enabled: bool) -> Result<()> {
    let (key, _) = RegKey::predef(HKEY_CURRENT_USER).create_subkey(KEY)?;
    if enabled {
        key.set_value(
            "ClashTray",
            &format!("\"{}\"", std::env::current_exe()?.display()),
        )?;
    } else {
        match key.delete_value("ClashTray") {
            Err(e) if e.kind() != std::io::ErrorKind::NotFound => return Err(e.into()),
            _ => {}
        }
    }
    Ok(())
}
