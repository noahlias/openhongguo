//! M1 proof: plays one URL inside a GPUI window.
//!
//!     cargo run --release --example play -- <url> [hex-key] [audio-url]
//!
//! space pause · ←/→ seek 5 s · ↑/↓ volume · f fullscreen · q quit.
//! Prints the dropped-frame count once a second.

use std::time::Duration;

use gpui_kit::*;
use ohg_desktop::player::{LoadRequest, Player};

struct Demo {
    player: Entity<Player>,
    focus: FocusHandle,
}

impl Render for Demo {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let s = self.player.read(cx).state.clone();
        div()
            .track_focus(&self.focus)
            .size_full()
            .bg(rgb(0x000000))
            .text_color(rgb(0xffffff))
            .on_key_down(cx.listener(|this, e: &KeyDownEvent, window, cx| {
                this.player
                    .update(cx, |p, _| match e.keystroke.key.as_str() {
                        "space" => p.toggle_pause(),
                        "left" => p.seek_by(-5.0),
                        "right" => p.seek_by(5.0),
                        "up" => p.set_volume(p.state.volume + 5.0),
                        "down" => p.set_volume(p.state.volume - 5.0),
                        _ => {}
                    });
                match e.keystroke.key.as_str() {
                    "f" => window.toggle_fullscreen(),
                    "q" => cx.quit(),
                    _ => {}
                }
            }))
            .child(div().absolute().size_full().child(self.player.clone()))
            .child(
                div()
                    .absolute()
                    .bottom_2()
                    .left_2()
                    .text_sm()
                    .child(format!(
                        "{:.1} / {:.1}  vol {:.0}{}",
                        s.position,
                        s.duration,
                        s.volume,
                        if s.paused { "  paused" } else { "" }
                    )),
            )
    }
}

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let Some(url) = args.first().cloned() else {
        eprintln!("usage: play <url> [hex-key] [audio-url]");
        std::process::exit(2);
    };
    let req = LoadRequest {
        url,
        key: args.get(1).cloned().unwrap_or_default(),
        audio_url: args.get(2).cloned().unwrap_or_default(),
        start: 0.0,
    };
    gpui_kit::application().run(move |cx| {
        gpui_kit::init(cx);
        let opts = WindowOptions {
            window_bounds: Some(WindowBounds::Windowed(Bounds::centered(
                None,
                size(px(540.), px(960.)),
                cx,
            ))),
            ..Default::default()
        };
        gpui_kit::open_window(opts, cx, |window, cx| {
            let player = Player::open(80.0, cx).expect("libmpv");
            player.update(cx, |p, cx| p.load(&req, cx).expect("load"));
            let focus = cx.focus_handle();
            window.focus(&focus, cx);
            let weak = player.downgrade();
            cx.spawn(async move |cx| {
                loop {
                    cx.background_executor().timer(Duration::from_secs(1)).await;
                    let Ok((dropped, pos, hw, frame)) = weak.update(cx, |p, _| {
                        (
                            p.dropped_frames(),
                            p.state.position,
                            p.hwdec_current(),
                            p.has_frame(),
                        )
                    }) else {
                        break;
                    };
                    eprintln!("t={pos:.1}s dropped={dropped} hwdec={hw} frame={frame}");
                }
            })
            .detach();
            cx.new(|cx| {
                cx.observe(&player, |_, _, cx| cx.notify()).detach();
                Demo { player, focus }
            })
        })
        .expect("window");
    });
}
