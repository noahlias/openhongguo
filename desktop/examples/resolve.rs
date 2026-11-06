//! Live check of the Go bridge: searches, lists episodes, resolves one.
//!
//!     cargo run --example resolve -- <query> [episode] [config-dir]
//!
//! Prints `URL KEY AUDIO` for the best stream, ready for the play example.

use ohg_desktop::sdk::Sdk;
use ohg_desktop::sdk::types::{
    Episode, EpisodeOptions, Page, PlaybackResult, SearchOptions, SeriesSummary,
};

fn main() -> anyhow::Result<()> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let query = args.first().cloned().unwrap_or_else(|| "全球死考".into());
    let number: i32 = args.get(1).and_then(|s| s.parse().ok()).unwrap_or(1);
    let dir = args
        .get(2)
        .map(Into::into)
        .unwrap_or_else(|| std::env::temp_dir().join("ohg-resolve"));
    let sdk = Sdk::open(&dir, "")?;
    futures::executor::block_on(async {
        let page: Page<SeriesSummary> = sdk
            .call(
                "search",
                (
                    &query,
                    SearchOptions {
                        page_size: 20,
                        ..Default::default()
                    },
                ),
            )
            .await?;
        let series = page
            .items
            .first()
            .ok_or_else(|| anyhow::anyhow!("no results"))?;
        eprintln!("series {} {}", series.id, series.title);
        let eps: Page<Episode> = sdk
            .call(
                "listEpisodes",
                (
                    &series.id,
                    EpisodeOptions {
                        page_size: 50,
                        ..Default::default()
                    },
                ),
            )
            .await?;
        let ep = eps
            .items
            .iter()
            .find(|e| e.number == number)
            .ok_or_else(|| anyhow::anyhow!("no episode {number}"))?;
        let pb: PlaybackResult = sdk.call("resolvePlaybackFresh", [&ep.id]).await?;
        let s = pb
            .playback
            .select("best")
            .ok_or_else(|| anyhow::anyhow!("no stream"))?;
        eprintln!(
            "{} {}x{} {} protected={}",
            s.definition, s.width, s.height, s.codec, pb.playback.protected
        );
        println!(
            "{} {} {}",
            s.url,
            pb.content_key.map(|k| k.key).unwrap_or_else(|| "-".into()),
            pb.playback.audio_url
        );
        Ok(())
    })
}
