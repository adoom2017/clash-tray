use anyhow::{Context, Result, bail};
use serde::Deserialize;
use std::{
    fs::{self, File},
    io::{Read, Write},
    path::Path,
    time::Duration,
};
use tempfile::TempDir;

#[derive(Deserialize)]
struct Release {
    tag_name: String,
    assets: Vec<Asset>,
}
#[derive(Deserialize)]
struct Asset {
    name: String,
    browser_download_url: String,
}
pub struct Prepared {
    pub version: String,
    pub directory: TempDir,
    pub unchanged: bool,
}

fn current_version(base: &Path) -> Option<String> {
    use std::{
        os::windows::process::CommandExt,
        process::{Command, Stdio},
        time::Instant,
    };
    let output = tempfile::tempfile().ok()?;
    let mut child = Command::new(base.join("mihomo.exe"))
        .arg("-v")
        .creation_flags(0x08000000)
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .stdout(output.try_clone().ok()?)
        .spawn()
        .ok()?;
    let deadline = Instant::now() + Duration::from_secs(5);
    loop {
        match child.try_wait() {
            Ok(Some(status)) if status.success() => break,
            Ok(None) if Instant::now() < deadline => std::thread::sleep(Duration::from_millis(50)),
            _ => {
                let _ = child.kill();
                let _ = child.wait();
                return None;
            }
        }
    }
    use std::io::{Seek, SeekFrom};
    let mut output = output;
    output.seek(SeekFrom::Start(0)).ok()?;
    let mut text = String::new();
    output.take(8192).read_to_string(&mut text).ok()?;
    text.split_whitespace()
        .find(|s| s.starts_with('v') && s.as_bytes().get(1).is_some_and(u8::is_ascii_digit))
        .map(str::to_owned)
}

fn asset_name(version: &str, arch: &str) -> Result<String> {
    let platform = match arch {
        "x86_64" => "amd64-compatible",
        "aarch64" => "arm64",
        "x86" => "386",
        _ => bail!("不支持的架构: {arch}"),
    };
    Ok(format!("mihomo-windows-{platform}-{version}.zip"))
}

pub fn prepare(base: &Path) -> Result<Prepared> {
    let client = reqwest::blocking::Client::builder()
        .user_agent("Clash-Tray/2.0")
        .connect_timeout(Duration::from_secs(20))
        .timeout(Duration::from_secs(180))
        .build()?;
    let release: Release = client
        .get("https://api.github.com/repos/MetaCubeX/mihomo/releases/latest")
        .send()?
        .error_for_status()?
        .json()?;
    let name = asset_name(&release.tag_name, std::env::consts::ARCH)?;
    let asset = release
        .assets
        .iter()
        .find(|a| a.name.eq_ignore_ascii_case(&name))
        .context("未找到适合当前架构的 Mihomo 安装包")?;
    let directory = tempfile::Builder::new()
        .prefix(".mihomo-update-")
        .tempdir_in(base)?;
    if current_version(base).as_deref() == Some(&release.tag_name) {
        return Ok(Prepared {
            version: release.tag_name,
            directory,
            unchanged: true,
        });
    }
    let archive = directory.path().join("release.zip");
    let mut response = client
        .get(&asset.browser_download_url)
        .send()?
        .error_for_status()?;
    let mut file = File::create(&archive)?;
    let size = std::io::copy(
        &mut Read::by_ref(&mut response).take(256 * 1024 * 1024 + 1),
        &mut file,
    )?;
    if size > 256 * 1024 * 1024 {
        bail!("下载包过大");
    }
    file.sync_all()?;
    drop(file);
    extract(&archive, &directory.path().join("mihomo.exe"))?;
    Ok(Prepared {
        version: release.tag_name,
        directory,
        unchanged: false,
    })
}

fn extract(archive: &Path, destination: &Path) -> Result<()> {
    let mut zip = zip::ZipArchive::new(File::open(archive)?)?;
    let mut selected = None;
    for i in 0..zip.len() {
        let entry = zip.by_index(i)?;
        let name = entry.name().to_ascii_lowercase();
        if !entry.is_dir()
            && name.starts_with("mihomo")
            && name.ends_with(".exe")
            && !name.contains(['/', '\\'])
        {
            if selected.is_some() {
                bail!("安装包包含多个 Mihomo 程序");
            }
            selected = Some(i);
        }
    }
    let mut entry = zip.by_index(selected.context("安装包中没有 Mihomo 程序")?)?;
    if entry.size() > 512 * 1024 * 1024 {
        bail!("解压文件过大");
    }
    let mut output = File::create(destination)?;
    let mut magic = [0; 2];
    entry.read_exact(&mut magic)?;
    if magic != *b"MZ" {
        bail!("无效的 Windows 可执行文件");
    }
    output.write_all(&magic)?;
    std::io::copy(&mut entry, &mut output)?;
    output.sync_all()?;
    Ok(())
}

pub fn install(prepared: &Prepared, base: &Path) -> Result<()> {
    replace(
        &prepared.directory.path().join("mihomo.exe"),
        &base.join("mihomo.exe"),
    )
}
fn replace(source: &Path, target: &Path) -> Result<()> {
    let backup = target.with_extension("exe.bak");
    let existed = target.exists();
    if existed {
        if backup.exists() {
            fs::remove_file(&backup).context("无法移除旧备份")?;
        }
        fs::rename(target, &backup).context("备份内核失败")?;
    }
    if let Err(error) = fs::rename(source, target) {
        if existed {
            fs::rename(&backup, target).with_context(|| {
                format!(
                    "替换失败 ({error})，恢复失败；备份位于 {}",
                    backup.display()
                )
            })?;
        }
        return Err(error).context("替换内核失败，已保留旧版本");
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    fn archive(path: &Path, name: &str, data: &[u8]) {
        let mut writer = zip::ZipWriter::new(File::create(path).unwrap());
        writer
            .start_file(name, zip::write::SimpleFileOptions::default())
            .unwrap();
        writer.write_all(data).unwrap();
        writer.finish().unwrap();
    }
    #[test]
    fn extracts_core_and_rejects_traversal_and_invalid_exe() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("test.zip");
        let target = dir.path().join("core.exe");
        archive(&path, "mihomo-windows-amd64.exe", b"MZtest");
        extract(&path, &target).unwrap();
        assert_eq!(fs::read(&target).unwrap(), b"MZtest");
        archive(&path, "../mihomo.exe", b"MZbad");
        assert!(extract(&path, &target).is_err());
        assert_eq!(fs::read(&target).unwrap(), b"MZtest");
        archive(&path, "mihomo.exe", b"NOT AN EXE");
        assert!(extract(&path, &target).is_err());
    }
    #[test]
    fn architecture_selection() {
        assert_eq!(
            asset_name("v1.2.3", "x86_64").unwrap(),
            "mihomo-windows-amd64-compatible-v1.2.3.zip"
        );
        assert!(asset_name("v1", "unknown").is_err());
    }
    #[test]
    fn failed_replacement_restores_old_core() {
        let dir = tempfile::tempdir().unwrap();
        let target = dir.path().join("mihomo.exe");
        fs::write(&target, b"old").unwrap();
        assert!(replace(&dir.path().join("missing"), &target).is_err());
        assert_eq!(fs::read(target).unwrap(), b"old");
    }
    #[test]
    fn replacement_keeps_backup() {
        let dir = tempfile::tempdir().unwrap();
        let target = dir.path().join("mihomo.exe");
        let source = dir.path().join("new.exe");
        fs::write(&target, b"old").unwrap();
        fs::write(&source, b"new").unwrap();
        replace(&source, &target).unwrap();
        assert_eq!(fs::read(&target).unwrap(), b"new");
        assert_eq!(fs::read(target.with_extension("exe.bak")).unwrap(), b"old");
    }
}
