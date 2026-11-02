package cliapp

import (
	"context"
	"errors"
	"fmt"

	"github.com/beilunyang/openhongguo/internal/playback"
	"github.com/beilunyang/openhongguo/sdk"
)

func cmdDownload(ctx context.Context, args []string, d Dependencies) error {
	if err := requireService(d); err != nil {
		return err
	}
	if d.Media == nil {
		return errors.New("media launcher is not configured")
	}
	f, err := parseFlags(args)
	if err != nil {
		return err
	}
	if len(f.rest) != 1 {
		return usage("download needs one series ID or title")
	}
	id, title, err := resolveSeries(ctx, d, f.rest[0], f.pick)
	if err != nil {
		return err
	}
	if series, err := d.Service.GetSeries(ctx, id); err == nil {
		if err := playback.RequireVideo(series); err != nil {
			return err
		}
		if title == "" {
			title = series.Title
		}
	}
	page, err := d.Service.ListEpisodes(ctx, id, sdk.EpisodeOptions{})
	if err != nil {
		return err
	}
	numbers, err := playback.ParseEpisodeRange(f.episodes, len(page.Items))
	if err != nil {
		return err
	}
	dir := f.dir
	if dir == "" {
		dir = playback.DefaultDir(title, id)
	}
	p := eventPrinter{d: d, f: f}
	job := playback.DownloadJob{
		Episodes: page.Items, Numbers: numbers, Dir: dir,
		Jobs: f.jobs, Quality: f.quality, OnRetry: p.downloadRetry,
	}
	failed, err := playback.Download(ctx, d.engine(), job, p.download)
	if err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d episodes failed; run the same command again to retry them", failed, len(numbers))
	}
	return nil
}
