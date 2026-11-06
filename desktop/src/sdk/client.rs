use std::ffi::{CStr, CString, c_char, c_void};
use std::future::Future;
use std::path::Path;
use std::ptr;
use std::sync::Arc;

use futures::channel::oneshot;
use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::Value;

use super::error::SdkError;
use super::ffi;
use super::trace::{Outcome, Trace};

/// The Go SDK, opened on a config directory shared with the `ohg` CLI.
/// Calls run on Go goroutines; each returns a future that cancels the Go
/// call when dropped unfinished.
pub struct Sdk {
    handle: u64,
    trace: Arc<Trace>,
}

impl Sdk {
    pub fn open(config_dir: &Path, config_json: &str) -> Result<Sdk, SdkError> {
        let dir = CString::new(config_dir.to_string_lossy().as_bytes()).map_err(SdkError::local)?;
        let cfg = CString::new(config_json).map_err(SdkError::local)?;
        let mut err: *mut c_char = ptr::null_mut();
        let handle = unsafe { ffi::ohg_open(dir.as_ptr(), cfg.as_ptr(), &mut err) };
        if handle == 0 {
            let msg = if err.is_null() {
                String::from("{}")
            } else {
                let s = unsafe { CStr::from_ptr(err) }
                    .to_string_lossy()
                    .into_owned();
                unsafe { ffi::ohg_free(err) };
                s
            };
            return Err(SdkError::from_json(&msg));
        }
        Ok(Sdk {
            handle,
            trace: Arc::default(),
        })
    }

    pub fn handle(&self) -> u64 {
        self.handle
    }

    pub fn trace(&self) -> &Trace {
        &self.trace
    }

    /// Calls `method` with `args` (serialized as the JSON argument array).
    pub fn call<T: DeserializeOwned + Send + 'static>(
        &self,
        method: &str,
        args: impl Serialize,
    ) -> impl Future<Output = Result<T, SdkError>> + Send + 'static {
        let (tx, rx) = oneshot::channel::<String>();
        let started = encode(method, args).map(|(c_method, c_args)| {
            let id = self.trace.begin(method, &c_args.to_string_lossy());
            let userdata = Box::into_raw(Box::new(tx)) as *mut c_void;
            let call = unsafe {
                ffi::ohg_call(
                    self.handle,
                    c_method.as_ptr(),
                    c_args.as_ptr(),
                    on_done,
                    userdata,
                )
            };
            CancelGuard {
                handle: self.handle,
                call,
                armed: true,
                trace: self.trace.clone(),
                id,
            }
        });
        async move {
            let mut guard = started?;
            let envelope = rx
                .await
                .map_err(|_| SdkError::local("the SDK dropped the call"))?;
            guard.armed = false;
            let result = decode(&envelope);
            guard.trace.finish(
                guard.id,
                match &result {
                    Ok(_) => Outcome::Ok(envelope.len()),
                    Err(e) => Outcome::Err(e.code.clone()),
                },
            );
            result
        }
    }
}

impl Drop for Sdk {
    fn drop(&mut self) {
        unsafe { ffi::ohg_close(self.handle) };
    }
}

fn encode(method: &str, args: impl Serialize) -> Result<(CString, CString), SdkError> {
    let args = serde_json::to_string(&args).map_err(SdkError::local)?;
    Ok((
        CString::new(method).map_err(SdkError::local)?,
        CString::new(args).map_err(SdkError::local)?,
    ))
}

fn decode<T: DeserializeOwned>(envelope: &str) -> Result<T, SdkError> {
    let mut v: Value = serde_json::from_str(envelope).map_err(SdkError::local)?;
    if let Some(err) = v.get("error") {
        return Err(SdkError::from_value(err));
    }
    let result = v.get_mut("result").map(Value::take).unwrap_or(Value::Null);
    serde_json::from_value(result).map_err(SdkError::local)
}

/// Runs on a Go thread exactly once per call. The string is valid only
/// during the callback, so it is copied before the channel send.
unsafe extern "C" fn on_done(userdata: *mut c_void, envelope: *const c_char) {
    let tx = unsafe { Box::from_raw(userdata as *mut oneshot::Sender<String>) };
    let s = unsafe { CStr::from_ptr(envelope) }
        .to_string_lossy()
        .into_owned();
    let _ = tx.send(s);
}

struct CancelGuard {
    handle: u64,
    call: u64,
    armed: bool,
    trace: Arc<Trace>,
    id: u64,
}

impl Drop for CancelGuard {
    fn drop(&mut self) {
        if self.armed {
            self.trace.finish(self.id, Outcome::Cancelled);
            if self.call != 0 {
                unsafe { ffi::ohg_cancel(self.handle, self.call) };
            }
        }
    }
}

// The Go side guards every handle and call with its own locks.
unsafe impl Send for Sdk {}
unsafe impl Sync for Sdk {}
