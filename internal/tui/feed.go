package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// feed is a list that grows a page at a time as the cursor nears its end,
// and asks for the highlighted series' details after the cursor rests.
type feed struct {
	list    listPane
	cursor  string
	hasMore bool
	loading bool
	req     int
	err     error
	// pending is the series whose details are wanted once the cursor rests.
	pending string
}

const detailDelay = 180 * time.Millisecond

func (f *feed) reset() {
	f.list.clear()
	f.cursor, f.hasMore, f.loading, f.err, f.pending = "", false, false, nil, ""
}

// begin marks a new request and returns its number.
func (f *feed) begin(s *seq) int {
	f.req = s.next()
	f.loading = true
	f.err = nil
	return f.req
}

// accept reports whether an answer belongs to the latest request.
func (f *feed) accept(req int) bool {
	if req != f.req {
		return false
	}
	f.loading = false
	return true
}

// page records one page's cursor and whether more follow.
func (f *feed) page(cursor string, hasMore bool) {
	f.cursor = cursor
	f.hasMore = hasMore && cursor != ""
}

// wantMore reports that the next page should be fetched now.
func (f *feed) wantMore() bool {
	return f.hasMore && !f.loading && f.list.nearEnd(5)
}

// afterMove schedules the detail fetch for the highlighted series. The
// caller passes the series ID under the cursor, or "" for a row without one.
func (f *feed) afterMove(e *env, id string) tea.Cmd {
	if id == "" || e.detail(id) != nil {
		f.pending = ""
		return nil
	}
	f.pending = id
	return tea.Tick(detailDelay, func(time.Time) tea.Msg { return detailTickMsg{id: id} })
}

// onDetailTick fetches the details if the cursor still rests on id.
func (f *feed) onDetailTick(e *env, id string) tea.Cmd {
	if f.pending != id || e.detail(id) != nil {
		return nil
	}
	f.pending = ""
	return e.loadDetail(id)
}

// selectedSeries returns the series ID under the cursor, if the row has one.
func (f *feed) selectedSeries() string {
	switch r := f.list.selected().(type) {
	case seriesRow:
		return r.item.ID
	case historyRow:
		return r.entry.SeriesID
	case workRow:
		return r.item.SeriesID
	case releaseRow:
		return r.item.SeriesID
	}
	return ""
}

// statusLine says what the feed is doing under the list.
func (f *feed) statusLine(tr *Strings, th *Theme) string {
	switch {
	case f.loading && f.list.len() == 0:
		return th.Subtle.Render(tr.Loading)
	case f.err != nil:
		return th.ToastErr.Render(describeError(f.err, tr))
	case f.loading:
		return th.Subtle.Render(tr.Loading)
	case f.list.len() == 0:
		return th.Subtle.Render(tr.Empty)
	}
	return ""
}
