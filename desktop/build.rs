//! Builds the Go SDK bridge (`bindings/cshared`) and links it, so a plain
//! `cargo build` needs only a Go toolchain (and on Windows a MinGW gcc for
//! cgo) on PATH.
//!
//! Linux and macOS link a static c-archive. On Windows, Go's archive is MinGW
//! code that does not link into an MSVC binary, so the bridge is a DLL
//! (`ohg.dll`) with an import library made by `lib.exe`, and the DLL is copied
//! next to the executable.

use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

const EXPORTS: [&str; 5] = [
    "ohg_open",
    "ohg_close",
    "ohg_call",
    "ohg_cancel",
    "ohg_free",
];

fn main() {
    let manifest = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let root = manifest
        .parent()
        .expect("desktop/ lives inside the repo")
        .to_path_buf();
    let out = PathBuf::from(env::var("OUT_DIR").unwrap());
    let target_os = env::var("CARGO_CFG_TARGET_OS").unwrap();

    if target_os == "windows" {
        let dll = out.join("ohg.dll");
        go_build(&root, "c-shared", &dll);
        import_lib(&out);
        println!("cargo:rustc-link-lib=dylib=ohg");
        // OUT_DIR is target/<profile>/build/<pkg>/out; the exe and tests run
        // from target/<profile> and target/<profile>/deps.
        let profile_dir = out.ancestors().nth(3).unwrap();
        for dir in [profile_dir.to_path_buf(), profile_dir.join("deps")] {
            fs::create_dir_all(&dir).unwrap();
            fs::copy(&dll, dir.join("ohg.dll")).unwrap();
        }
    } else {
        go_build(&root, "c-archive", &out.join("libohg.a"));
        println!("cargo:rustc-link-lib=static=ohg");
        println!("cargo:rustc-link-lib=dylib=resolv");
        if target_os == "macos" {
            println!("cargo:rustc-link-lib=framework=CoreFoundation");
            println!("cargo:rustc-link-lib=framework=Security");
        }
    }
    println!("cargo:rustc-link-search=native={}", out.display());

    for dir in [
        "sdk",
        "adapters",
        "internal",
        "bindings/internal",
        "bindings/cshared",
        "go.mod",
        "go.sum",
    ] {
        println!("cargo:rerun-if-changed={}", root.join(dir).display());
    }
    println!("cargo:rerun-if-env-changed=GO");
}

fn go_build(root: &Path, mode: &str, output: &Path) {
    let go = env::var("GO").unwrap_or_else(|_| "go".into());
    let status = Command::new(&go)
        .current_dir(root)
        .env("CGO_ENABLED", "1")
        .args(["build", &format!("-buildmode={mode}"), "-trimpath", "-o"])
        .arg(output)
        .arg("./bindings/cshared")
        .status()
        .unwrap_or_else(|e| panic!("running `{go} build` failed: {e} (is Go installed?)"));
    assert!(status.success(), "go build of bindings/cshared failed");
}

fn import_lib(out: &Path) {
    let def = out.join("ohg.def");
    fs::write(
        &def,
        format!("LIBRARY ohg.dll\nEXPORTS\n{}\n", EXPORTS.join("\n")),
    )
    .unwrap();
    let target = env::var("TARGET").unwrap();
    let mut lib = cc::windows_registry::find(&target, "lib.exe")
        .expect("lib.exe not found: build from an MSVC toolchain");
    let machine = match env::var("CARGO_CFG_TARGET_ARCH").unwrap().as_str() {
        "aarch64" => "ARM64",
        _ => "X64",
    };
    let status = lib
        .arg(format!("/DEF:{}", def.display()))
        .arg(format!("/OUT:{}", out.join("ohg.lib").display()))
        .arg(format!("/MACHINE:{machine}"))
        .status()
        .expect("run lib.exe");
    assert!(status.success(), "lib.exe failed to make ohg.lib");
}
