use anyhow::{Context, Result, bail};
use std::{
    io::Write,
    os::windows::process::CommandExt,
    process::{Command, Stdio},
};
use winreg::{RegKey, enums::*};
const KEY: &str = r"Software\Microsoft\Windows\CurrentVersion\Run";

fn run_script(
    source: &str,
    mode: &str,
    executable: &std::path::Path,
) -> Result<std::process::Output> {
    let mut script = tempfile::Builder::new().suffix(".ps1").tempfile()?;
    // Windows PowerShell 5.1 requires a BOM for Unicode script text.
    script.write_all(b"\xef\xbb\xbf")?;
    script.write_all(source.trim_start_matches('\u{feff}').as_bytes())?;
    script.flush()?;
    // Close the file handle before PowerShell reads it; TempPath still removes it on drop.
    let script = script.into_temp_path();
    let powershell = std::env::var_os("SystemRoot").context("SystemRoot is missing")?;
    let output = Command::new(
        std::path::PathBuf::from(powershell).join("System32/WindowsPowerShell/v1.0/powershell.exe"),
    )
    .args([
        "-NoLogo",
        "-NoProfile",
        "-NonInteractive",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
    ])
    // Set UTF-8 before loading the file, so even file-loading errors are readable.
    // Paths and values are data in the child environment, never interpolated as code.
    .arg(r#"$ErrorActionPreference = 'Stop'; [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false); try { & $env:CLASH_TRAY_SCRIPT -Mode $env:CLASH_TRAY_MODE -Executable $env:CLASH_TRAY_EXECUTABLE; if ($LASTEXITCODE) { exit $LASTEXITCODE } } catch { [Console]::Error.Write($_.Exception.Message); exit 1 }"#)
    .env("CLASH_TRAY_SCRIPT", script.as_os_str())
    .env("CLASH_TRAY_MODE", mode)
    .env("CLASH_TRAY_EXECUTABLE", executable)
    .creation_flags(0x08000000)
    .stdin(Stdio::null())
    .output()
    .context("无法连接 Windows 任务计划程序")?;
    Ok(output)
}

fn scheduled_task(mode: &str) -> Result<bool> {
    let output = run_script(
        include_str!("autostart.ps1"),
        mode,
        &std::env::current_exe()?,
    )?;
    if !output.status.success() {
        bail!(
            "设置或查询计划任务失败: {}",
            String::from_utf8_lossy(&output.stderr).trim()
        );
    }
    match String::from_utf8_lossy(&output.stdout).trim() {
        "enabled" => Ok(true),
        "disabled" => Ok(false),
        other => bail!("计划任务返回了未知状态: {other}"),
    }
}

pub fn enabled() -> Result<bool> {
    scheduled_task("query")
}

fn legacy_enabled() -> Result<bool> {
    let key = match RegKey::predef(HKEY_CURRENT_USER).open_subkey(KEY) {
        Ok(key) => key,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(false),
        Err(e) => return Err(e.into()),
    };
    let value: String = match key.get_value("ClashTray") {
        Ok(value) => value,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(false),
        Err(e) => return Err(e.into()),
    };
    Ok(value
        .trim_matches('"')
        .eq_ignore_ascii_case(&std::env::current_exe()?.to_string_lossy()))
}

fn remove_legacy() -> Result<()> {
    let key = match RegKey::predef(HKEY_CURRENT_USER).open_subkey_with_flags(KEY, KEY_SET_VALUE) {
        Ok(key) => key,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(()),
        Err(e) => return Err(e.into()),
    };
    match key.delete_value("ClashTray") {
        Err(e) if e.kind() != std::io::ErrorKind::NotFound => Err(e.into()),
        _ => Ok(()),
    }
}

pub fn initialize() -> Result<bool> {
    if legacy_enabled()? {
        set(true)
    } else {
        enabled()
    }
}

pub fn set(enable: bool) -> Result<bool> {
    let actual = scheduled_task(if enable { "enable" } else { "disable" })?;
    remove_legacy().context("计划任务已更新，但清理旧自启动项失败")?;
    Ok(actual)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn powershell_reads_closed_temp_script_and_preserves_arguments() {
        let path = std::path::Path::new(r"C:\中文 & spaces\it's $value.exe");
        let output = run_script(
            "param($Mode, $Executable) [Console]::Write($Mode + '|' + $Executable)",
            "query",
            path,
        )
        .unwrap();
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        assert_eq!(
            String::from_utf8(output.stdout).unwrap(),
            format!("query|{}", path.display())
        );
    }

    #[test]
    fn powershell_errors_are_utf8() {
        let output = run_script(
            "param($Mode, $Executable) throw '自启动测试：无法访问文件'",
            "query",
            std::path::Path::new("unused.exe"),
        )
        .unwrap();
        assert!(!output.status.success());
        assert!(
            String::from_utf8(output.stderr)
                .unwrap()
                .contains("自启动测试：无法访问文件")
        );
    }

    #[test]
    fn powershell_preserves_script_exit_code() {
        let output = run_script(
            "param($Mode, $Executable) [Console]::Error.Write('计划任务注册失败'); exit 7",
            "enable",
            std::path::Path::new("unused.exe"),
        )
        .unwrap();
        assert_eq!(output.status.code(), Some(7));
        assert_eq!(
            String::from_utf8(output.stderr).unwrap(),
            "计划任务注册失败"
        );
    }
}
