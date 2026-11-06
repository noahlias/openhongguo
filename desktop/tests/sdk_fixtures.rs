//! Round-trips the Go bridge against the fixture replay server
//! (`bindings/mobile/fixtureserver`, which serves `testdata/protocol`).

use std::io::{BufRead, BufReader};
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};

use ohg_desktop::sdk::Sdk;
use ohg_desktop::sdk::types::{
    HistoryEntry, Page, PlaybackResult, ResumePoint, SearchOptions, SeriesSummary,
};

struct Server(Child);

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.0.kill();
    }
}

fn fixture_server() -> (Server, String) {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .unwrap()
        .to_path_buf();
    // `go run` would leave the built server running after a kill, so build it first.
    let bin = std::env::temp_dir().join(format!("ohg-fixtureserver-{}", std::process::id()));
    let built = Command::new("go")
        .current_dir(&root)
        .args(["build", "-o"])
        .arg(&bin)
        .arg("./bindings/mobile/fixtureserver")
        .status()
        .expect("go build fixtureserver");
    assert!(built.success());
    let mut child = Command::new(&bin)
        .current_dir(&root)
        .args(["-dir", "testdata/protocol"])
        .stdout(Stdio::piped())
        .spawn()
        .expect("start fixtureserver");
    let mut line = String::new();
    BufReader::new(child.stdout.take().unwrap())
        .read_line(&mut line)
        .unwrap();
    (Server(child), line.trim().to_string())
}

#[test]
fn bridge_round_trip() {
    let (_srv, url) = fixture_server();
    let dir = std::env::temp_dir().join(format!("ohg-desktop-test-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    let cfg = serde_json::json!({"apiOrigin": url, "registerOrigin": url, "retryAttempts": 1})
        .to_string();
    let sdk = Sdk::open(&dir, &cfg).expect("open");

    futures::executor::block_on(async {
        let page: Page<SeriesSummary> = sdk
            .call(
                "search",
                (
                    "短剧",
                    SearchOptions {
                        page_size: 20,
                        ..Default::default()
                    },
                ),
            )
            .await
            .unwrap();
        assert!(!page.items.is_empty());
        assert!(!page.items[0].id.is_empty());

        let pb: PlaybackResult = sdk
            .call("resolvePlaybackFresh", ["7100000000000201"])
            .await
            .unwrap();
        assert!(pb.playback.protected);
        assert!(!pb.playback.usable().is_empty());

        let err = sdk.call::<serde_json::Value>("nope", ()).await.unwrap_err();
        assert_eq!(err.code, "INVALID_OPTIONS");

        let entry = HistoryEntry {
            series_id: "s1".into(),
            episode: 2,
            episodes: 5,
            position: 40.0,
            ..Default::default()
        };
        let _: serde_json::Value = sdk.call("historyRecord", [&entry]).await.unwrap();
        let list: Vec<HistoryEntry> = sdk.call("historyList", ()).await.unwrap();
        assert_eq!(list.len(), 1);
        assert_eq!(list[0].position, 40.0);
        let rp: ResumePoint = sdk.call("resumePoint", ("s1", 5)).await.unwrap();
        assert_eq!((rp.episode, rp.start), (2, 37));
    });
    assert!(dir.join("anonymous.json").exists());
    assert!(dir.join("history.json").exists());
    drop(sdk);
    let _ = std::fs::remove_dir_all(&dir);
}
