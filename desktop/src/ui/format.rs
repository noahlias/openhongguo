//! Ports of `internal/tui/format.go` and `labels.go`.

/// 1.2万 / 3.4亿 in Chinese, 12.3K / 1.2M in English; zero is empty.
pub fn count(n: i64, zh: bool) -> String {
    if n <= 0 {
        return String::new();
    }
    let units: &[(i64, &str)] = if zh {
        &[(100_000_000, "亿"), (10_000, "万")]
    } else {
        &[(1_000_000_000, "B"), (1_000_000, "M"), (1_000, "K")]
    };
    for &(div, unit) in units {
        if n >= div {
            return format!("{}{unit}", trim_zero(n as f64 / div as f64));
        }
    }
    n.to_string()
}

fn trim_zero(f: f64) -> String {
    let s = format!("{f:.1}");
    s.strip_suffix(".0").map(str::to_string).unwrap_or(s)
}

/// m:ss or h:mm:ss.
pub fn clock(seconds: f64) -> String {
    let s = if seconds.is_finite() && seconds > 0.0 {
        (seconds + 0.5) as i64
    } else {
        0
    };
    if s >= 3600 {
        format!("{}:{:02}:{:02}", s / 3600, s % 3600 / 60, s % 60)
    } else {
        format!("{}:{:02}", s / 60, s % 60)
    }
}

pub fn status(s: i32, zh: bool) -> &'static str {
    match (s, zh) {
        (0, true) => "更新中",
        (1, true) => "已完结",
        (3, true) => "今日更新",
        (4, true) => "已停更",
        (0, false) => "Updating",
        (1, false) => "Finished",
        (3, false) => "Updated today",
        (4, false) => "Stopped",
        _ => "",
    }
}

pub fn pay(p: i32, zh: bool) -> &'static str {
    match (p, zh) {
        (0, true) => "免费",
        (1, true) => "会员",
        (2, true) => "单点付费",
        (0, false) => "Free",
        (1, false) => "VIP",
        (2, false) => "Pay per title",
        _ => "",
    }
}

pub fn content(c: i32, zh: bool) -> &'static str {
    let (z, e) = match c {
        1 => ("短剧", "Short drama"),
        2 => ("情景剧", "Scene play"),
        3 => ("达人推荐", "Creator pick"),
        4 => ("图文推荐", "Slideshow"),
        5 => ("电影", "Movie"),
        6 => ("电视剧", "TV series"),
        7 => ("小短剧", "Short play"),
        1002 => ("AI 短剧", "AI drama"),
        1003 => ("合集", "Album"),
        1004 => ("漫剧", "Motion comic"),
        _ => return "",
    };
    if zh { z } else { e }
}

/// The ISO-8601 `updated_at` of a history entry, as a short local-looking date.
pub fn date(iso: &str) -> String {
    iso.get(..16)
        .map(|s| s.replace('T', " "))
        .unwrap_or_else(|| iso.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn counts_match_the_tui() {
        assert_eq!(count(0, true), "");
        assert_eq!(count(9_999, true), "9999");
        assert_eq!(count(12_000, true), "1.2万");
        assert_eq!(count(100_000, true), "10万");
        assert_eq!(count(340_000_000, true), "3.4亿");
        assert_eq!(count(12_300, false), "12.3K");
        assert_eq!(count(1_200_000, false), "1.2M");
    }

    #[test]
    fn clocks() {
        assert_eq!(clock(0.0), "0:00");
        assert_eq!(clock(65.4), "1:05");
        assert_eq!(clock(3725.0), "1:02:05");
        assert_eq!(clock(f64::NAN), "0:00");
    }
}
