package cliapp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk"
	"github.com/beilunyang/openhongguo/sdk/model"
)

func cmdPlay(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("play needs one series ID or title")
	}
	if f.url && f.download != "" {
		return usage("--url and --download are mutually exclusive")
	}
	if f.download != "" && f.player != "" && f.player != "auto" {
		return usage("--download does not use --player")
	}
	key := strings.TrimSpace(f.key)
	if key == "" {
		key = strings.TrimSpace(os.Getenv("OHG_CENC_KEY"))
	}
	if key != "" && f.url {
		return usage("--key cannot be combined with --url")
	}
	if key != "" && f.noDecrypt {
		return usage("--key and --no-decrypt are mutually exclusive")
	}
	if f.autoplay && (f.url || f.json || f.download != "") {
		return usage("--autoplay cannot be combined with --url, --json, or --download")
	}
	if f.autoplay && (f.player == "open" || f.player == "xdg-open") {
		return usage("--autoplay needs mpv, ffplay, or vlc to know when an episode ends")
	}
	if f.autoplay && key != "" {
		return usage("--autoplay derives each episode's key; drop --key and OHG_CENC_KEY")
	}
	if f.resume && f.episodeSet {
		return usage("--resume and --episode are mutually exclusive")
	}
	if f.resume && (f.url || f.json || f.download != "") {
		return usage("--resume cannot be combined with --url, --json, or --download")
	}
	if f.resume && d.History == nil {
		return errors.New("watch history is not configured")
	}
	id, title, err := resolveSeries(ctx, d, f.rest[0], f.pick)
	if err != nil {
		return err
	}
	page, err := d.Service.ListEpisodes(ctx, id, sdk.EpisodeOptions{})
	if err != nil {
		return err
	}
	start := 0
	if f.resume {
		n, pos, err := playback.ResumePoint(ctx, d.History, id, len(page.Items))
		if err != nil {
			return err
		}
		f.episode, start = n, pos
	}
	episode, ok := playback.FindEpisode(page.Items, f.episode)
	if !ok {
		return usage("series %s has no episode %d", id, f.episode)
	}
	// Only a real playback is remembered, not --url, --json, --download, or
	// a --preview that never reaches the end.
	remember := d.History != nil && !f.url && !f.json && f.download == "" && f.preview == 0
	if series, err := d.Service.GetSeries(ctx, id); err == nil {
		if err := playback.RequireVideo(series); err != nil {
			return err
		}
		if title == "" {
			title = series.Title
		}
	}
	if f.json || f.url || f.download != "" {
		return playOnce(ctx, d, f, episode, key)
	}
	p := eventPrinter{d: d, f: f}
	opts := playback.Options{
		Quality: f.quality, Player: f.player, Key: key,
		Autoplay: f.autoplay, Preview: f.preview, NoDecrypt: f.noDecrypt,
	}
	return playback.NewSession(d.engine(), opts, p.event).Run(ctx, playback.Target{
		SeriesID: id, Title: title, Episodes: page.Items,
		Start: episode, StartSeconds: start, Remember: remember,
	})
}

// playOnce reports or saves one resolved episode for --json, --url, and
// --download, which never play.
func playOnce(ctx context.Context, d Dependencies, f flags, episode model.Episode, key string) error {
	pb, err := playback.Resolve(ctx, d.engine(), episode.ID)
	if err != nil {
		return err
	}
	p, err := playback.Prepare(pb, episode, f.quality, key, f.noDecrypt)
	if err != nil {
		return err
	}
	if f.json {
		if err := printValue(d.Stdout, pb, true); err != nil {
			return err
		}
		if pb.Protected && p.Key == "" {
			return playback.DRMError(pb)
		}
		return nil
	}
	if f.url {
		// The CDN URLs are public and expire on their own, so --url reports
		// them without touching the decryptor.
		fmt.Fprintln(d.Stdout, p.Stream.URL)
		if pb.AudioURL != "" {
			// A segmented title keeps its audio in a separate track.
			fmt.Fprintln(d.Stdout, pb.AudioURL)
		}
		return nil
	}
	if pb.Protected && p.Key == "" {
		return playback.DRMError(pb)
	}
	if d.Media == nil {
		return errors.New("media launcher is not configured")
	}
	printer := eventPrinter{d: d, f: f}
	written, err := playback.WithBackup(ctx, p.Stream, printer.retry, func(url string) (int64, error) {
		return playback.SaveStream(ctx, d.Media, url, pb.AudioURL, p.Key, pb.Protected, f.download, f.preview)
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Stdout, "%s\t%d\t%s\n", f.download, written, streamName(p.Stream))
	return nil
}

// resolveSeries turns a series ID or a title into a series ID. A title is
// searched: an exact title match wins, then the first result; with pick the
// viewer chooses. The title is returned when the search found it.
func resolveSeries(ctx context.Context, d Dependencies, arg string, pick bool) (id, title string, err error) {
	if isNumeric(arg) {
		if pick {
			return "", "", usage("--pick needs a title, not a series ID")
		}
		id, err := validateID(arg)
		return id, "", err
	}
	if strings.Contains(arg, "://") {
		_, err := validateID(arg)
		return "", "", err
	}
	query := strings.TrimSpace(arg)
	if query == "" {
		return "", "", usage("missing series ID or title")
	}
	page, err := d.Service.Search(ctx, query, sdk.SearchOptions{PageSize: 10})
	if err != nil {
		return "", "", usageFrom(err)
	}
	if len(page.Items) == 0 {
		return "", "", fmt.Errorf("no series matches %q", query)
	}
	choice := page.Items[0]
	if pick {
		choice, err = pickSeries(d, page.Items)
		if err != nil {
			return "", "", err
		}
	} else {
		for _, s := range page.Items {
			if s.Title == query {
				choice = s
				break
			}
		}
		fmt.Fprintf(d.Stderr, "ohg: %s (%s, %d episodes)\n", choice.Title, choice.ID, choice.EpisodeCount)
	}
	return choice.ID, choice.Title, nil
}

// pickSeries lists items on stderr and reads the chosen number from stdin.
func pickSeries(d Dependencies, items []model.SeriesSummary) (model.SeriesSummary, error) {
	for i, s := range items {
		fmt.Fprintf(d.Stderr, "%2d) %s (%d episodes)\n", i+1, s.Title, s.EpisodeCount)
	}
	fmt.Fprintf(d.Stderr, "choose 1-%d: ", len(items))
	if d.Stdin == nil {
		return model.SeriesSummary{}, errors.New("--pick needs an interactive stdin")
	}
	line, err := bufio.NewReader(d.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return model.SeriesSummary{}, usage("no series chosen")
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(items) {
		return model.SeriesSummary{}, usage("choose a number from 1 to %d", len(items))
	}
	return items[n-1], nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// cmdContinue resumes the series played most recently.
func cmdContinue(ctx context.Context, args []string, d Dependencies) error {
	if d.History == nil {
		return errors.New("watch history is not configured")
	}
	entries, err := d.History.List(ctx)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return usage("nothing to continue; play a series first")
	}
	return cmdPlay(ctx, append([]string{entries[0].SeriesID, "--resume"}, args...), d)
}
