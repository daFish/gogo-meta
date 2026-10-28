//nolint:errcheck // Terminal output write errors are intentionally ignored.
package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

const (
	progressInterval = 100 * time.Millisecond
	clearLine        = "\r\x1b[K"

	// maxDetailRunes caps the detail when the terminal width is unknown.
	maxDetailRunes = 40
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Progress draws a single self-overwriting status line on ErrWriter while a
// long-running phase works, so the user can tell the command has not stalled.
// It draws nothing unless ErrWriter is a terminal, which keeps piped output,
// CI logs and tests free of control sequences. A nil *Progress is a no-op.
//
// Output printed between StartProgress and Stop has to go through Print, or
// the redraw would land in the middle of it.
type Progress struct {
	w        io.Writer
	width    func() int
	label    string
	total    int
	start    time.Time
	interval time.Duration

	mu     sync.Mutex
	done   int
	frame  int
	detail string
	drawn  bool

	stop    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

// StartProgress shows label with a spinner. With a positive total it also
// shows how many of the total items have completed; call Advance once per item.
func StartProgress(label string, total int) *Progress {
	return newProgress(ErrWriter, IsInteractive(ErrWriter), label, total, progressInterval)
}

// IsInteractive reports whether the progress line draws on w. Override in
// tests to see the line without a terminal.
var IsInteractive = isTerminal

func newProgress(w io.Writer, enabled bool, label string, total int, interval time.Duration) *Progress {
	p := &Progress{
		w:        w,
		width:    func() int { return terminalWidth(w) },
		label:    label,
		total:    total,
		start:    time.Now(),
		interval: interval,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	if !enabled {
		close(p.stopped)
		return p
	}
	go p.run()
	return p
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// terminalWidth returns the number of columns of the terminal w writes to, or
// 0 when it is unknown.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return width
}

// Advance marks one item as completed. It is safe for concurrent use.
func (p *Progress) Advance() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.done++
	p.mu.Unlock()
}

// SetDetail shows detail after the counter, such as the item the phase is
// waiting on. An empty detail removes it. It is safe for concurrent use.
func (p *Progress) SetDetail(detail string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.detail = sanitizeText(detail)
	p.mu.Unlock()
}

// Print runs printBlock, which writes a block of output, with the status line
// removed, and draws the line again below the block. No redraw can land in the
// middle of the block. printBlock must not call methods of p, which would
// deadlock.
func (p *Progress) Print(printBlock func()) {
	if p == nil {
		printBlock()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.drawn {
		printBlock()
		return
	}
	fmt.Fprint(p.w, clearLine)
	printBlock()
	p.render()
}

// Stop removes the status line. It returns once the line is cleared, so the
// caller can print immediately afterwards. Calling it more than once is safe.
func (p *Progress) Stop() {
	if p == nil {
		return
	}
	p.once.Do(func() { close(p.stop) })
	<-p.stopped
}

func (p *Progress) run() {
	defer close(p.stopped)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// The first frame waits one tick, so a phase that finishes almost at once
	// leaves no flicker behind.
	for {
		select {
		case <-p.stop:
			p.clear()
			return
		case <-ticker.C:
			p.draw()
		}
	}
}

func (p *Progress) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.drawn {
		fmt.Fprint(p.w, clearLine)
		p.drawn = false
	}
}

func (p *Progress) draw() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.render()
	p.frame++
	p.drawn = true
}

// render writes the status line; the caller holds p.mu. The line never
// reaches the last column of the terminal: once it wraps, clearLine only clears
// its last row and every redraw leaves the rows above it behind. The detail is
// shortened first, from the left, so the end of a long project path stays
// visible.
func (p *Progress) render() {
	frame := spinnerFrames[p.frame%len(spinnerFrames)]

	counter := ""
	if p.total > 0 {
		counter = fmt.Sprintf(" %d/%d", p.done, p.total)
	}
	elapsed := fmt.Sprintf("(%s)", time.Since(p.start).Truncate(time.Second))
	head := frame + " " + p.label + counter

	width := p.width()
	detailBudget := maxDetailRunes
	if width > 0 {
		detailBudget = width - 1 - utf8.RuneCountInString(head+", "+" "+elapsed)
	}
	detail := ""
	if clipped := clipLeft(p.detail, detailBudget); clipped != "" {
		detail = ", " + clipped
	}

	if line := head + detail + " " + elapsed; width > 0 && utf8.RuneCountInString(line) >= width {
		fmt.Fprint(p.w, clearLine+clipRight(line, width-1))
		return
	}
	fmt.Fprintf(p.w, "%s%s %s%s%s %s", clearLine, cyanFn(frame), p.label, counter, detail, dimFn(elapsed))
}

// clipLeft shortens s to at most n runes by replacing its beginning with "…".
func clipLeft(s string, n int) string {
	runes := []rune(s)
	switch {
	case len(runes) <= n:
		return s
	case n <= 0:
		return ""
	}
	return "…" + string(runes[len(runes)-n+1:])
}

// clipRight shortens s to at most n runes by cutting its end.
func clipRight(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:max(n, 0)])
}
