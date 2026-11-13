// The Go core through the host app: one call by name with a JSON argument array, answered with the
// same {result} | {error} envelope as the desktop's ohg_call (bindings/internal/desktop).
import { host } from "nsh";

/** An SDK error as the Go dispatcher encodes it. */
export class OhgError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly retryable = false,
    readonly retryAfterMs = 0,
  ) {
    super(message);
  }

  /** The service will not hand this anonymous device a stream right now. */
  get unavailable() {
    return ["ANONYMOUS_LIMITED", "AUTHENTICATION_REQUIRED", "DRM_PROTECTED", "CONTENT_UNAVAILABLE"].includes(this.code);
  }
}

type Envelope = { result?: unknown; error?: { code?: string; message?: string; retryable?: boolean; retryAfterMs?: number } };

let nextId = 0;

/** Calls `method` of the Go core; an abort cancels the Go call, which then rejects with CANCELLED. */
export async function call<T>(method: string, args: unknown[] = [], options: { signal?: AbortSignal } = {}): Promise<T> {
  const id = ++nextId;
  const abort = () => host.post("ohg.cancel", { id });
  options.signal?.addEventListener("abort", abort);
  try {
    const raw = await host.call("ohg.call", { id, method, args: JSON.stringify(args) }, { timeout: 0 });
    const env = JSON.parse(raw as string) as Envelope;
    if (env.error) {
      const e = env.error;
      throw new OhgError(e.code ?? "LOCAL", e.message ?? "", e.retryable ?? false, e.retryAfterMs ?? 0);
    }
    return env.result as T;
  } finally {
    options.signal?.removeEventListener("abort", abort);
  }
}

const described: Record<string, [string, string]> = {
  CANCELLED: ["已取消", "cancelled"],
  RATE_LIMITED: ["请求太频繁，请稍后再试", "rate limited, try again later"],
  TRANSPORT: ["网络错误", "network error"],
  ANONYMOUS_LIMITED: ["匿名设备暂时拿不到播放地址，请稍后再试", "the anonymous device got no stream, try later"],
  AUTHENTICATION_REQUIRED: ["需要登录才能观看", "this needs an account"],
  DEVICE_REJECTED: ["设备被拒绝，请在设置里重置设备", "device rejected, reset it in Settings"],
  CHALLENGE_REQUIRED: ["服务要求人机验证，请稍后再试", "the service wants a challenge, try later"],
  SIGNATURE_REJECTED: ["签名被拒绝", "signature rejected"],
  CONTENT_UNAVAILABLE: ["内容不可用", "content unavailable"],
  END_OF_LIST: ["没有更多了", "no more items"],
};

/** Short user-facing text, a port of `SdkError::describe` (internal/tui/fetch.go). */
export function describe(err: unknown, zh: boolean): string {
  if (err instanceof OhgError) {
    const d = described[err.code];
    return d ? d[zh ? 0 : 1] : err.message;
  }
  return err instanceof Error ? err.message : String(err);
}
