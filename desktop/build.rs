//! Builds the Go SDK bridge (`bindings/cshared`) as a static C archive and
//! links it, so a plain `cargo build` needs only a Go toolchain on PATH.

use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() {
    let manifest = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let root = manifest
        .parent()
        .expect("desktop/ lives inside the repo")
        .to_path_buf();
    let out = PathBuf::from(env::var("OUT_DIR").unwrap());
    let lib = out.join("libohg.a");

    let go = env::var("GO").unwrap_or_else(|_| "go".into());
    let status = Command::new(&go)
        .current_dir(&root)
        .env("CGO_ENABLED", "1")
        .args(["build", "-buildmode=c-archive", "-trimpath", "-o"])
        .arg(&lib)
        .arg("./bindings/cshared")
        .status()
        .unwrap_or_else(|e| panic!("running `{go} build` failed: {e} (is Go installed?)"));
    assert!(status.success(), "go build of bindings/cshared failed");

    println!("cargo:rustc-link-search=native={}", out.display());
    println!("cargo:rustc-link-lib=static=ohg");
    println!("cargo:rustc-link-lib=dylib=resolv");
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
