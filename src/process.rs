use anyhow::{Context, Result};
use std::{
    fs::OpenOptions,
    io::{Read, Write},
    os::windows::process::CommandExt,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
    thread,
};

#[derive(Clone)]
pub struct Logger(Arc<Mutex<PathBuf>>);
impl Logger {
    pub fn new(path: PathBuf) -> Result<Self> {
        OpenOptions::new().create(true).append(true).open(&path)?;
        Ok(Self(Arc::new(Mutex::new(path))))
    }
    pub fn write(&self, data: &[u8]) {
        let path = self.0.lock().unwrap_or_else(|e| e.into_inner());
        if std::fs::metadata(&*path)
            .map(|m| m.len() > 10 * 1024 * 1024)
            .unwrap_or(false)
        {
            let backup = path.with_extension("log.1");
            let _ = std::fs::remove_file(&backup);
            let _ = std::fs::rename(&*path, backup);
        }
        if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&*path) {
            let _ = file.write_all(data);
        }
    }
    fn pipe(&self, mut reader: impl Read + Send + 'static) {
        let log = self.clone();
        thread::spawn(move || {
            let mut buf = [0; 8192];
            loop {
                match reader.read(&mut buf) {
                    Ok(0) | Err(_) => break,
                    Ok(n) => log.write(&buf[..n]),
                }
            }
        });
    }
}

#[derive(Default)]
pub struct Core {
    child: Option<Child>,
}
impl Core {
    pub fn running(&self) -> bool {
        self.child.is_some()
    }
    pub fn start(&mut self, base: &Path, log: &Logger) -> Result<()> {
        if self.running() {
            return Ok(());
        }
        let mut child = Command::new(base.join("mihomo.exe"))
            .arg("-d")
            .arg(base.join("config"))
            .current_dir(base)
            .creation_flags(0x08000000)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .context("启动 Mihomo 失败")?;
        log.pipe(child.stdout.take().unwrap());
        log.pipe(child.stderr.take().unwrap());
        self.child = Some(child);
        Ok(())
    }
    pub fn poll(&mut self, log: &Logger) -> Result<()> {
        if let Some(child) = self.child.as_mut()
            && let Some(status) = child.try_wait()?
        {
            log.write(format!("\nMihomo exited: {status}\n").as_bytes());
            self.child = None;
        }
        Ok(())
    }
    pub fn stop(&mut self) -> Result<()> {
        if let Some(child) = self.child.as_mut() {
            if child.try_wait()?.is_none() {
                child.kill()?;
            }
            child.wait()?;
            self.child = None;
        }
        Ok(())
    }
}
impl Drop for Core {
    fn drop(&mut self) {
        let _ = self.stop();
    }
}
