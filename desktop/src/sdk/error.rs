use std::fmt;

use serde::Deserialize;
use serde_json::Value;

/// An SDK error as the Go dispatcher encodes it: `{code, message, retryable, retryAfterMs}`.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SdkError {
    #[serde(default)]
    pub code: String,
    #[serde(default)]
    pub message: String,
    #[serde(default)]
    pub retryable: bool,
    #[serde(default)]
    pub retry_after_ms: i64,
}

impl SdkError {
    pub fn local(err: impl fmt::Display) -> SdkError {
        SdkError {
            code: "LOCAL".into(),
            message: err.to_string(),
            retryable: false,
            retry_after_ms: 0,
        }
    }

    pub fn from_value(v: &Value) -> SdkError {
        serde_json::from_value(v.clone()).unwrap_or_else(|_| SdkError::local(v))
    }

    pub fn from_json(s: &str) -> SdkError {
        serde_json::from_str(s).unwrap_or_else(|_| SdkError::local(s))
    }

    pub fn is(&self, code: &str) -> bool {
        self.code == code
    }

    /// The service will not hand this anonymous device a stream right now.
    pub fn is_unavailable(&self) -> bool {
        matches!(
            self.code.as_str(),
            "ANONYMOUS_LIMITED"
                | "AUTHENTICATION_REQUIRED"
                | "DRM_PROTECTED"
                | "CONTENT_UNAVAILABLE"
        )
    }

    /// Short user-facing text, a port of `describeError` in `internal/tui/fetch.go`.
    pub fn describe(&self, zh: bool) -> String {
        let (z, e) = match self.code.as_str() {
            "CANCELLED" => ("已取消", "cancelled"),
            "RATE_LIMITED" => ("请求太频繁，请稍后再试", "rate limited, try again later"),
            "TRANSPORT" => ("网络错误", "network error"),
            "ANONYMOUS_LIMITED" => (
                "匿名设备暂时拿不到播放地址，请稍后再试",
                "the anonymous device got no stream, try later",
            ),
            "AUTHENTICATION_REQUIRED" => ("需要登录才能观看", "this needs an account"),
            "DEVICE_REJECTED" => (
                "设备被拒绝，请在设置里重置设备",
                "device rejected, reset it in Settings",
            ),
            "CHALLENGE_REQUIRED" => (
                "服务要求人机验证，请稍后再试",
                "the service wants a challenge, try later",
            ),
            "SIGNATURE_REJECTED" => ("签名被拒绝", "signature rejected"),
            "CONTENT_UNAVAILABLE" => ("内容不可用", "content unavailable"),
            "END_OF_LIST" => ("没有更多了", "no more items"),
            _ => return self.message.clone(),
        };
        if zh { z.into() } else { e.into() }
    }
}

impl fmt::Display for SdkError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.code, self.message)
    }
}

impl std::error::Error for SdkError {}
