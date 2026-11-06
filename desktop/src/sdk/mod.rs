//! The Go SDK over its C ABI (`bindings/cshared`).

mod client;
mod error;
mod ffi;
pub mod trace;
pub mod types;

pub use client::Sdk;
pub use error::SdkError;
