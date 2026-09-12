//! Convert RGBA PNG artwork into multi-resolution Windows ICO files.
//! cargo run --example prepare_icons -- assets/icons/running.png app-enable.ico
use anyhow::{Context, Result, ensure};
use image::{GenericImageView, ImageFormat, imageops::FilterType};
use std::{
    fs::File,
    io::{Cursor, Write},
};

fn main() -> Result<()> {
    let args: Vec<_> = std::env::args().collect();
    ensure!(args.len() == 3, "Usage: prepare_icons INPUT.png OUTPUT.ico");
    let source = image::open(&args[1]).context("Unable to read icon artwork")?;
    let (w, h) = source.dimensions();
    ensure!(w == h, "Icon artwork must be square");
    let sizes = [16u32, 20, 24, 32, 40, 48, 64, 128, 256];
    let mut frames = Vec::new();
    for size in sizes {
        let rgba = source
            .resize_exact(size, size, FilterType::Lanczos3)
            .into_rgba8();
        let mut png = Cursor::new(Vec::new());
        rgba.write_to(&mut png, ImageFormat::Png)?;
        frames.push(png.into_inner());
    }
    let mut output = File::create(&args[2])?;
    output.write_all(&[0, 0, 1, 0])?;
    output.write_all(&(sizes.len() as u16).to_le_bytes())?;
    let mut offset = 6 + sizes.len() as u32 * 16;
    for (size, frame) in sizes.iter().zip(&frames) {
        let dimension = if *size == 256 { 0 } else { *size as u8 };
        output.write_all(&[dimension, dimension, 0, 0])?;
        output.write_all(&1u16.to_le_bytes())?;
        output.write_all(&32u16.to_le_bytes())?;
        output.write_all(&(frame.len() as u32).to_le_bytes())?;
        output.write_all(&offset.to_le_bytes())?;
        offset += frame.len() as u32;
    }
    for frame in frames {
        output.write_all(&frame)?;
    }
    Ok(())
}
