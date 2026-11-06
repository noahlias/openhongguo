//! A record of every call that crossed the FFI bridge, for the debug panel.

use std::collections::VecDeque;
use std::sync::Mutex;
use std::time::{Duration, Instant};

const KEEP: usize = 200;
const ARGS_MAX: usize = 160;

#[derive(Debug, Clone, PartialEq)]
pub enum Outcome {
    Pending,
    /// The envelope's size in bytes.
    Ok(usize),
    /// The SDK error code.
    Err(String),
    Cancelled,
}

#[derive(Debug, Clone)]
pub struct CallRecord {
    pub id: u64,
    pub method: String,
    pub args: String,
    pub started: Instant,
    pub elapsed: Option<Duration>,
    pub outcome: Outcome,
}

#[derive(Debug, Clone, Default)]
pub struct Snapshot {
    pub total: u64,
    pub errors: u64,
    pub in_flight: usize,
    /// Newest last.
    pub recent: Vec<CallRecord>,
}

#[derive(Default)]
pub struct Trace {
    inner: Mutex<Inner>,
}

#[derive(Default)]
struct Inner {
    next: u64,
    total: u64,
    errors: u64,
    in_flight: usize,
    recent: VecDeque<CallRecord>,
}

impl Trace {
    pub fn begin(&self, method: &str, args: &str) -> u64 {
        let mut s = self.inner.lock().unwrap();
        s.next += 1;
        s.total += 1;
        s.in_flight += 1;
        let id = s.next;
        if s.recent.len() == KEEP {
            s.recent.pop_front();
        }
        s.recent.push_back(CallRecord {
            id,
            method: method.to_string(),
            args: truncate(args),
            started: Instant::now(),
            elapsed: None,
            outcome: Outcome::Pending,
        });
        id
    }

    pub fn finish(&self, id: u64, outcome: Outcome) {
        let mut s = self.inner.lock().unwrap();
        s.in_flight = s.in_flight.saturating_sub(1);
        if matches!(outcome, Outcome::Err(_)) {
            s.errors += 1;
        }
        if let Some(r) = s.recent.iter_mut().rev().find(|r| r.id == id) {
            r.elapsed = Some(r.started.elapsed());
            r.outcome = outcome;
        }
    }

    pub fn snapshot(&self) -> Snapshot {
        let s = self.inner.lock().unwrap();
        Snapshot {
            total: s.total,
            errors: s.errors,
            in_flight: s.in_flight,
            recent: s.recent.iter().cloned().collect(),
        }
    }
}

fn truncate(args: &str) -> String {
    match args.char_indices().nth(ARGS_MAX) {
        Some((i, _)) => format!("{}…", &args[..i]),
        None => args.to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::{ARGS_MAX, Outcome, Trace};

    #[test]
    fn counts_outcomes_and_keeps_the_newest() {
        let t = Trace::default();
        let a = t.begin("discover", "[{}]");
        let b = t.begin("getSeries", "[\"1\"]");
        let c = t.begin("search", &"长".repeat(ARGS_MAX + 5));
        t.finish(a, Outcome::Ok(42));
        t.finish(b, Outcome::Err("NOT_FOUND".into()));
        let s = t.snapshot();
        assert_eq!((s.total, s.errors, s.in_flight), (3, 1, 1));
        assert_eq!(s.recent[0].outcome, Outcome::Ok(42));
        assert!(s.recent[0].elapsed.is_some());
        assert_eq!(s.recent[2].outcome, Outcome::Pending);
        assert_eq!(s.recent[2].args.chars().count(), ARGS_MAX + 1);
        t.finish(c, Outcome::Cancelled);
        assert_eq!(t.snapshot().in_flight, 0);
    }
}
