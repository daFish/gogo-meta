//nolint:errcheck // Terminal output write errors are intentionally ignored.
package output

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

const (
	progressInterval = 100 * time.Millisecond
	clearLine        = "\r\x1b[K"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Progress draws a single self-overwriting status line on ErrWriter while a
// long-running phase works, so the user can tell the command has not stalled.
// It draws nothing unless ErrWriter is a terminal, which keeps piped output,
// CI logs and tests free of control sequences. A nil *Progress is a no-op.
//
// Nothing else may be printed between StartProgress and Stop, or the redraw
// would land in the middle of that output.
type Progress struct {
	w        io.Writer
	label    string
	total    int
	start    time.Time
	interval time.Duration

	mu    sync.Mutex
	done  int
	frame int

	stop    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

// StartProgress shows label with a spinner. With a positive total it also
// shows how many of the total items have completed; call Advance once per item.
func StartProgress(label string, total int) *Progress {
	return newProgress(ErrWriter, isInteractive(ErrWriter), label, total, progressInterval)
}

func newProgress(w io.Writer, enabled bool, label string, total int, interval time.Duration) *Progress {
	p := &Progress{
		w:        w,
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

func isInteractive(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
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
	drawn := false
	for {
		select {
		case <-p.stop:
			if drawn {
				fmt.Fprint(p.w, clearLine)
			}
			return
		case <-ticker.C:
			p.draw()
			drawn = true
		}
	}
}

func (p *Progress) draw() {
	p.mu.Lock()
	frame := spinnerFrames[p.frame%len(spinnerFrames)]
	p.frame++
	done := p.done
	p.mu.Unlock()

	counter := ""
	if p.total > 0 {
		counter = fmt.Sprintf(" %d/%d", done, p.total)
	}
	elapsed := time.Since(p.start).Truncate(time.Second)

	fmt.Fprintf(p.w, "%s%s %s%s %s", clearLine, cyanFn(frame), p.label, counter, dimFn(fmt.Sprintf("(%s)", elapsed)))
}
