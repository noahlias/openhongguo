//! The window's root: sidebar, the section page with series pages stacked on
//! top, the theater over everything while playing, and the toasts.

use std::time::Duration;

use gpui_kit::assets::IconName;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;

use super::debug::{DebugPanel, PANEL_W, ToggleDebug};
use super::icons::icon;
use super::layout::Layout;
use super::motion;
use super::pages::discover::DiscoverPage;
use super::pages::library::{LibraryPage, Mode};
use super::pages::search::SearchPage;
use super::pages::series::SeriesPage;
use super::pages::settings::SettingsPage;
use super::state::{self, Nav, PlayRequest};
use super::theater::Theater;
use super::theme::palette;
use crate::player::Player;

const TOAST_FOR: Duration = Duration::from_millis(3500);

mod nav;

#[derive(Clone, Copy, PartialEq)]
enum Section {
    Continue,
    Discover,
    Search,
    History,
    Settings,
}

struct Toast {
    id: usize,
    text: SharedString,
    error: bool,
}

pub struct AppView {
    section: Section,
    continue_: Entity<LibraryPage>,
    discover: Entity<DiscoverPage>,
    search: Entity<SearchPage>,
    history: Entity<LibraryPage>,
    settings: Entity<SettingsPage>,
    stack: Vec<Entity<SeriesPage>>,
    theater: Option<Entity<Theater>>,
    /// One libmpv instance for the app's life, opened on the first play.
    player: Option<Entity<Player>>,
    toasts: Vec<Toast>,
    next_toast: usize,
    focus: FocusHandle,
    save_size: Option<Task<()>>,
    navigated: bool,
    debug: Option<Entity<DebugPanel>>,
}

impl AppView {
    pub fn new(window: &mut Window, cx: &mut Context<Self>) -> AppView {
        let continue_ = cx.new(|cx| LibraryPage::new(Mode::Continue, cx));
        let discover = cx.new(DiscoverPage::new);
        let search = cx.new(|cx| SearchPage::new(window, cx));
        let history = cx.new(|cx| LibraryPage::new(Mode::History, cx));
        let settings = cx.new(|_| SettingsPage::default());
        cx.subscribe_in(&continue_, window, Self::on_nav).detach();
        cx.subscribe_in(&discover, window, Self::on_nav).detach();
        cx.subscribe_in(&search, window, Self::on_nav).detach();
        cx.subscribe_in(&history, window, Self::on_nav).detach();
        cx.subscribe_in(&settings, window, Self::on_nav).detach();
        cx.observe_window_bounds(window, |this, window, cx| this.remember_size(window, cx))
            .detach();
        // History loads after the window opens; start on Continue once it has
        // something, unless the user already went somewhere.
        cx.observe_global::<state::Ohg>(|this: &mut AppView, cx| {
            if !this.navigated
                && this.section == Section::Discover
                && !state::ohg(cx).history.is_empty()
            {
                this.section = Section::Continue;
                cx.notify();
            }
            this.sync_debug(cx);
        })
        .detach();
        let section = if state::ohg(cx).history.is_empty() {
            Section::Discover
        } else {
            Section::Continue
        };
        let focus = cx.focus_handle();
        window.focus(&focus, cx);
        let mut view = AppView {
            section,
            continue_,
            discover,
            search,
            history,
            settings,
            stack: Vec::new(),
            theater: None,
            player: None,
            toasts: Vec::new(),
            next_toast: 0,
            focus,
            save_size: None,
            navigated: false,
            debug: None,
        };
        view.sync_debug(cx);
        view
    }

    /// The panel exists only while `settings.debug` is on, so its sampling
    /// timer stops with it.
    fn sync_debug(&mut self, cx: &mut Context<Self>) {
        let on = state::ohg(cx).settings.debug;
        if on && self.debug.is_none() {
            let player = self.player.clone();
            self.debug = Some(cx.new(|cx| DebugPanel::new(player, cx)));
            cx.notify();
        } else if !on && self.debug.take().is_some() {
            cx.notify();
        }
    }

    fn toggle_debug(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if let Err(err) = state::update_settings(window, cx, |s| s.debug = !s.debug) {
            self.toast(format!("{err:#}"), true, cx);
        }
        self.sync_debug(cx);
    }

    fn on_nav<E>(&mut self, _: &Entity<E>, nav: &Nav, window: &mut Window, cx: &mut Context<Self>) {
        match nav.clone() {
            Nav::Open(summary) => {
                self.navigated = true;
                if self
                    .stack
                    .last()
                    .is_some_and(|s| s.read(cx).id() == summary.id)
                {
                    return;
                }
                let page = cx.new(|cx| SeriesPage::new(summary, cx));
                cx.subscribe_in(&page, window, Self::on_nav).detach();
                self.stack.push(page);
            }
            Nav::Back => {
                if self.theater.take().is_none() {
                    self.stack.pop();
                }
                window.focus(&self.focus, cx);
            }
            Nav::Play(req) => self.play(req, window, cx),
            Nav::Toast { text, error } => self.toast(text, error, cx),
        }
        cx.notify();
    }

    fn play(&mut self, req: PlayRequest, window: &mut Window, cx: &mut Context<Self>) {
        let player = match &self.player {
            Some(p) => p.clone(),
            None => match Player::open(state::ohg(cx).settings.volume, cx) {
                Ok(p) => {
                    self.player = Some(p.clone());
                    if let Some(d) = &self.debug {
                        d.update(cx, |d, _| d.player = Some(p.clone()));
                    }
                    p
                }
                Err(err) => return self.toast(format!("libmpv: {err:#}"), true, cx),
            },
        };
        let theater = cx.new(|cx| Theater::new(req, player, window, cx));
        cx.subscribe_in(&theater, window, Self::on_nav).detach();
        window.focus(&theater.focus_handle(cx), cx);
        self.theater = Some(theater);
    }

    fn toast(&mut self, text: impl Into<SharedString>, error: bool, cx: &mut Context<Self>) {
        let id = self.next_toast;
        self.next_toast += 1;
        self.toasts.push(Toast {
            id,
            text: text.into(),
            error,
        });
        self.toasts.drain(..self.toasts.len().saturating_sub(4));
        cx.spawn(async move |this, cx| {
            cx.background_executor().timer(TOAST_FOR).await;
            let _ = this.update(cx, |v, cx| {
                v.toasts.retain(|t| t.id != id);
                cx.notify();
            });
        })
        .detach();
        cx.notify();
    }

    fn go(&mut self, section: Section, window: &mut Window, cx: &mut Context<Self>) {
        self.section = section;
        self.navigated = true;
        self.stack.clear();
        if section == Section::Search {
            self.search.update(cx, |s, cx| s.focus(window, cx));
        } else {
            window.focus(&self.focus, cx);
        }
        cx.notify();
    }

    /// Saved half a second after the last resize; fullscreen is not a size.
    fn remember_size(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        if window.is_fullscreen() {
            return;
        }
        let size = window.window_bounds().get_bounds().size;
        let (w, h) = (
            f32::from(size.width).round(),
            f32::from(size.height).round(),
        );
        self.save_size = Some(cx.spawn_in(window, async move |this, cx| {
            cx.background_executor()
                .timer(Duration::from_millis(500))
                .await;
            let _ = this.update_in(cx, |v, window, cx| {
                if let Err(err) = state::update_settings(window, cx, |s| {
                    s.window.width = w;
                    s.window.height = h;
                }) {
                    v.toast(format!("{err:#}"), true, cx);
                }
            });
        }));
    }

    fn content(&self) -> AnyView {
        if let Some(page) = self.stack.last() {
            return page.clone().into();
        }
        match self.section {
            Section::Continue => self.continue_.clone().into(),
            Section::Discover => self.discover.clone().into(),
            Section::Search => self.search.clone().into(),
            Section::History => self.history.clone().into(),
            Section::Settings => self.settings.clone().into(),
        }
    }

    /// The current page, easing in whenever it changes.
    fn page(&self) -> impl IntoElement {
        let view = self.content();
        let id = ("page", view.entity_id());
        motion::enter(div().size_full().child(view), id, 280, Duration::ZERO, 10.)
    }
}

impl Render for AppView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        for old in state::covers(cx).update(cx, |c, _| c.take_retired()) {
            let _ = window.drop_image(old);
        }
        let p = palette(cx);
        let t = state::tr(cx);
        let layout = Layout::current(window, cx);
        let toasts = div()
            .absolute()
            .bottom(px(20.))
            .right(px(20.))
            .flex()
            .flex_col()
            .items_end()
            .gap(px(8.))
            .children(self.toasts.iter().map(|toast| {
                let id = ("toast", toast.id);
                let el = div()
                    .max_w(px(420.))
                    .px(px(16.))
                    .py(px(10.))
                    .rounded(px(12.))
                    .bg(p.raised)
                    .border_1()
                    .border_color(if toast.error {
                        p.accent.opacity(0.6)
                    } else {
                        p.line
                    })
                    .shadow_lg()
                    .flex()
                    .items_center()
                    .gap(px(10.))
                    .child(icon(
                        if toast.error {
                            IconName::CircleAlert
                        } else {
                            IconName::CircleCheck
                        },
                        16.,
                        if toast.error { p.accent } else { p.green },
                    ))
                    .child(div().text_sm().text_color(p.fg).child(toast.text.clone()));
                motion::enter(el, id, 320, Duration::ZERO, 12.)
            }));
        div()
            .id("app")
            .track_focus(&self.focus)
            .size_full()
            .relative()
            .flex()
            .bg(p.bg)
            .text_color(p.fg)
            .on_action(
                cx.listener(|this, _: &ToggleDebug, window, cx| this.toggle_debug(window, cx)),
            )
            .on_key_down(cx.listener(|this, e: &KeyDownEvent, window, cx| {
                let k = &e.keystroke;
                if k.modifiers.secondary(cx) && k.key == "k" && this.theater.is_none() {
                    this.go(Section::Search, window, cx);
                } else if k.key == "escape" && this.theater.is_none() && !this.stack.is_empty() {
                    this.stack.pop();
                    cx.notify();
                }
            }))
            .child(
                div()
                    .flex_1()
                    .min_w_0()
                    .h_full()
                    .relative()
                    .flex()
                    .when(layout != Layout::Compact, |d| {
                        d.child(self.sidebar(t, p, layout, cx))
                    })
                    .child(
                        div()
                            .flex_1()
                            .min_w_0()
                            .h_full()
                            .flex()
                            .flex_col()
                            .child(div().flex_1().min_h_0().w_full().child(self.page()))
                            .when(layout == Layout::Compact, |d| {
                                d.child(self.tab_bar(t, p, cx))
                            }),
                    )
                    .when_some(self.theater.clone(), |d, th| {
                        let id = ("theater", th.entity_id());
                        d.child(div().absolute().inset_0().child(motion::enter(
                            div().size_full().child(th),
                            id,
                            240,
                            Duration::ZERO,
                            0.,
                        )))
                    }),
            )
            .when_some(self.debug.clone(), |d, panel| {
                d.child(motion::enter(
                    div().flex_none().h_full().w(px(PANEL_W)).child(panel),
                    "debug-in",
                    200,
                    Duration::ZERO,
                    0.,
                ))
            })
            .child(toasts)
    }
}
