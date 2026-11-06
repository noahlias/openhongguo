//! The in-window video player: libmpv decoding, BGRA software rendering into
//! GPUI's image atlas, and the episode session on top.

pub mod engine;
pub mod surface;

pub use engine::LoadRequest;
pub use surface::{PlayState, Player, PlayerEvent};
