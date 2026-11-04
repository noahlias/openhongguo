package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	_ "golang.org/x/image/webp"
)

const (
	coverMaxBytes = 2 << 20
	coverEntries  = 64
	coverCols     = 20
	coverMaxRows  = 14
	// coverMaxPixels refuses images whose decoded size dwarfs their bytes.
	coverMaxPixels = 4096 * 4096
	// Kitty image IDs are sent as a 24-bit foreground color, so they start
	// well above the small IDs other programs tend to use.
	kittyIDBase = 0x4F4800
)

type graphicsMode int

const (
	graphicsBlocks graphicsMode = iota
	graphicsKitty
)

// kittyQueryID marks the query the protocol uses for detection: a terminal
// that speaks it answers with this ID, any other ignores the query.
const kittyQueryID = 31

var kittyQuery = ansi.KittyGraphics([]byte("AAAA"), fmt.Sprintf("i=%d", kittyQueryID), "s=1,v=1,a=q,t=d,f=24")

// isKittyAnswer reports whether ev is the terminal accepting the query.
func isKittyAnswer(ev uv.KittyGraphicsEvent) bool {
	return ev.Options.ID == kittyQueryID && string(ev.Payload) == "OK"
}

// coverMsg answers a cover fetch with the decoded cover.
type coverMsg struct {
	url  string
	id   int
	rows int
	// img is sized for kitty, ten pixels per column and twenty per row.
	img *image.RGBA
	err error
}

// drawnCover is a cover ready for the frame.
type drawnCover struct {
	coverMsg
	// cells is what the frame shows: half-blocks, or blank cells whose
	// first carries place.
	cells string
	place string
}

// covers keeps the drawn covers, oldest out first. A URL that failed is
// remembered so it is not fetched again.
type covers struct {
	mode   graphicsMode
	client *http.Client
	// ffmpeg decodes the HEIC covers the service serves; without it only
	// JPEG, PNG and WebP covers are drawn.
	ffmpeg  string
	mu      sync.Mutex
	nextID  int
	drawn   map[string]drawnCover
	loading map[string]bool
	failed  map[string]bool
	order   []string
}

// newCovers draws half-blocks until useKitty is called.
func newCovers(client *http.Client) *covers {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		ffmpeg = ""
	}
	return &covers{
		client: client, ffmpeg: ffmpeg, drawn: map[string]drawnCover{},
		loading: map[string]bool{}, failed: map[string]bool{},
	}
}

// fetch starts loading url unless it is loaded, loading or failed.
func (c *covers) fetch(ctx context.Context, url string) tea.Cmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	if url == "" || c.loading[url] || c.failed[url] {
		return nil
	}
	if _, ok := c.drawn[url]; ok {
		return nil
	}
	c.loading[url] = true
	c.nextID++
	id := kittyIDBase + c.nextID
	client, ffmpeg := c.client, c.ffmpeg
	return func() tea.Msg {
		msg, err := prepareCover(ctx, client, ffmpeg, url, id)
		if err != nil {
			return coverMsg{url: url, err: err}
		}
		return msg
	}
}

// store keeps a fetched cover and returns what must be written to the
// terminal: the image to transmit, and the images to forget.
func (c *covers) store(msg coverMsg) tea.Cmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.loading, msg.url)
	if msg.err != nil {
		c.failed[msg.url] = true
		return nil
	}
	d, transmit, err := c.render(msg)
	if err != nil {
		c.failed[msg.url] = true
		return nil
	}
	c.drawn[msg.url] = d
	c.order = append(c.order, msg.url)
	var raw strings.Builder
	raw.WriteString(transmit)
	for len(c.order) > coverEntries {
		old := c.drawn[c.order[0]]
		delete(c.drawn, c.order[0])
		c.order = c.order[1:]
		if c.mode == graphicsKitty {
			raw.WriteString(kittyDelete(old.id))
		}
	}
	if raw.Len() == 0 {
		return nil
	}
	return tea.Raw(raw.String())
}

// useKitty switches to kitty images once the terminal answered the query,
// and sends the covers already drawn as half-blocks.
func (c *covers) useKitty() tea.Cmd {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mode == graphicsKitty {
		return nil
	}
	c.mode = graphicsKitty
	var raw strings.Builder
	for url, d := range c.drawn {
		redrawn, transmit, err := c.render(d.coverMsg)
		if err != nil {
			delete(c.drawn, url)
			c.failed[url] = true
			continue
		}
		c.drawn[url] = redrawn
		raw.WriteString(transmit)
	}
	c.order = slices.DeleteFunc(c.order, func(url string) bool { _, ok := c.drawn[url]; return !ok })
	if raw.Len() == 0 {
		return nil
	}
	return tea.Raw(raw.String())
}

// render lays out msg for the current mode; transmit is what a kitty
// terminal must receive before the cells show the image.
func (c *covers) render(msg coverMsg) (d drawnCover, transmit string, err error) {
	d.coverMsg = msg
	if c.mode == graphicsBlocks {
		d.cells = halfBlocks(shrink(msg.img, coverCols, 2*msg.rows))
		return d, "", nil
	}
	var out strings.Builder
	err = kitty.EncodeGraphics(&out, msg.img, &kitty.Options{
		Action: kitty.Transmit, Transmission: kitty.Direct, Format: kitty.PNG, ID: msg.id, Quiet: 2, Chunk: true,
	})
	if err != nil {
		return d, "", err
	}
	d.place = kittyPlace(msg.id, msg.rows)
	blank := strings.Repeat(" ", coverCols)
	d.cells = d.place + blank + strings.Repeat("\n"+blank, msg.rows-1)
	return d, out.String(), nil
}

// isPlacement reports whether seq places one of the drawn covers.
func (c *covers) isPlacement(seq string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.drawn {
		if d.place == seq {
			return true
		}
	}
	return false
}

func (c *covers) kitty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mode == graphicsKitty
}

// draw returns the cover's lines, or nil when it is not loaded.
func (c *covers) draw(url string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.drawn[url]
	if !ok {
		return nil
	}
	return strings.Split(m.cells, "\n")
}

// forget is written on exit so a kitty terminal frees the images.
func (c *covers) forget() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mode != graphicsKitty {
		return ""
	}
	var b strings.Builder
	for _, m := range c.drawn {
		b.WriteString(kittyDelete(m.id))
	}
	return b.String()
}

// kittyClear removes every image placed on screen but keeps the images.
var kittyClear = ansi.KittyGraphics(nil, "a=d", "d=a", "q=2")

// kittyPlace puts image id at the cursor without moving it, so the
// renderer's idea of the cursor stays right.
func kittyPlace(id, rows int) string {
	return ansi.KittyGraphics(nil, "a=p", fmt.Sprintf("i=%d,p=1,c=%d,r=%d,C=1,q=2", id, coverCols, rows))
}

func kittyDelete(id int) string {
	return ansi.KittyGraphics(nil, "a=d", fmt.Sprintf("d=I,i=%d", id), "q=2")
}

func prepareCover(ctx context.Context, client *http.Client, ffmpeg, url string, id int) (coverMsg, error) {
	img, err := getImage(ctx, client, ffmpeg, url)
	if err != nil {
		return coverMsg{}, err
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return coverMsg{}, fmt.Errorf("cover: empty image")
	}
	// A cell is about twice as tall as it is wide.
	rows := min(max(coverCols*b.Dy()/b.Dx()/2, 1), coverMaxRows)
	return coverMsg{url: url, id: id, rows: rows, img: shrink(img, coverCols*10, rows*20)}, nil
}

func getImage(ctx context.Context, client *http.Client, ffmpeg, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, coverMaxBytes))
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case err == nil:
		if cfg.Width*cfg.Height > coverMaxPixels {
			return nil, fmt.Errorf("cover: image is %dx%d", cfg.Width, cfg.Height)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		return img, err
	case errors.Is(err, image.ErrFormat) && ffmpeg != "" && isISOMedia(data):
		return decodeWithFFmpeg(ctx, ffmpeg, data)
	}
	return nil, err
}

// isISOMedia reports whether data starts like an HEIC file: an ISO media
// box of type ftyp.
func isISOMedia(data []byte) bool {
	return len(data) >= 12 && string(data[4:8]) == "ftyp"
}

// decodeWithFFmpeg turns a format Go cannot read (HEIC) into a small PNG.
// The input is a temporary file, not a pipe: ffmpeg seeks back to the
// image data after reading the boxes. The demuxer is named, drefs are off
// and only files are allowed, so data cannot make ffmpeg open other files
// or URLs the way a probed playlist could.
func decodeWithFFmpeg(ctx context.Context, ffmpeg string, data []byte) (image.Image, error) {
	f, err := os.CreateTemp("", "ohg-cover-*.heic")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error",
		"-protocol_whitelist", "file", "-f", "mov", "-enable_drefs", "0", "-i", f.Name(), "-frames:v", "1",
		"-vf", "scale=240:720:force_original_aspect_ratio=decrease", "-f", "image2pipe", "-c:v", "png", "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil && len(out) == 0 {
		err = errors.New("no image")
	}
	if err != nil {
		return nil, fmt.Errorf("cover: ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return png.Decode(bytes.NewReader(out))
}

// halfBlocks draws img one cell per two stacked pixels: the upper pixel is
// the foreground of ▀ and the lower one its background.
func halfBlocks(img *image.RGBA) string {
	b := img.Bounds()
	var out strings.Builder
	for y := 0; y+1 < b.Dy(); y += 2 {
		if y > 0 {
			out.WriteByte('\n')
		}
		for x := 0; x < b.Dx(); x++ {
			t, u := img.RGBAAt(x, y), img.RGBAAt(x, y+1)
			fmt.Fprintf(&out, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", t.R, t.G, t.B, u.R, u.G, u.B)
		}
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

// shrink box-averages img down to w×h.
func shrink(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for gy := 0; gy < h; gy++ {
		y0 := b.Min.Y + gy*b.Dy()/h
		y1 := max(b.Min.Y+(gy+1)*b.Dy()/h, y0+1)
		for gx := 0; gx < w; gx++ {
			x0 := b.Min.X + gx*b.Dx()/w
			x1 := max(b.Min.X+(gx+1)*b.Dx()/w, x0+1)
			var r, g, bl, n uint64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, _ := img.At(x, y).RGBA()
					r, g, bl, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), n+1
				}
			}
			out.SetRGBA(gx, gy, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 0xff})
		}
	}
	return out
}
