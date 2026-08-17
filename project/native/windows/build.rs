use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() {
    println!("cargo:rerun-if-changed=resources.rc");
    println!("cargo:rerun-if-changed=assets/AppIcon.ico");

    let target = env::var("TARGET").unwrap_or_default();
    if target != "x86_64-pc-windows-gnu" {
        return;
    }

    let output = PathBuf::from(env::var_os("OUT_DIR").expect("OUT_DIR is required"))
        .join("zcode-antigravity-resources.o");
    let windres = env::var("WINDRES").unwrap_or_else(|_| "x86_64-w64-mingw32-windres".into());
    let status = Command::new(windres)
        .args(["--input-format=rc", "--output-format=coff", "resources.rc"])
        .arg(&output)
        .status()
        .expect("failed to start windres");
    assert!(status.success(), "windres failed");
    println!("cargo:rustc-link-arg={}", output.display());
}
