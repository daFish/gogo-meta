package output

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgress(t *testing.T) {
	t.Run("draws while running and clears the line on Stop", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Pulling", 3, time.Millisecond)
		time.Sleep(50 * time.Millisecond)
		p.Stop()

		out := buf.String()
		assert.Contains(t, out, "Pulling 0/3")
		assert.True(t, strings.HasSuffix(out, clearLine), "Stop must clear the status line")
	})

	t.Run("shows completed items against the total", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Pulling", 3, time.Millisecond)
		p.Advance()
		p.draw()

		assert.Contains(t, buf.String(), "Pulling 1/3")
	})

	t.Run("omits the counter without a total", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Inspecting working copy", 0, time.Millisecond)
		p.draw()

		assert.Contains(t, buf.String(), "Inspecting working copy (")
		assert.NotContains(t, buf.String(), "/0")
	})

	t.Run("writes nothing after Stop returns", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Cloning", 1, time.Millisecond)
		time.Sleep(10 * time.Millisecond)
		p.Stop()
		written := buf.Len()

		time.Sleep(20 * time.Millisecond)
		assert.Equal(t, written, buf.Len())
	})

	t.Run("writes nothing when stopped before the first tick", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Cloning", 1, time.Hour)
		p.Stop()

		assert.Empty(t, buf.String())
	})

	t.Run("writes nothing when disabled", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Pulling", 2, time.Millisecond)
		p.Advance()
		time.Sleep(10 * time.Millisecond)
		p.Stop()

		assert.Empty(t, buf.String())
	})

	t.Run("is disabled when ErrWriter is not a terminal", func(t *testing.T) {
		_, stderr := captureOutput(func() {
			p := StartProgress("Pulling", 2)
			p.Advance()
			p.Stop()
		})
		assert.Empty(t, stderr)
	})

	t.Run("Stop is idempotent and a nil progress is a no-op", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Pulling", 1, time.Millisecond)
		p.Stop()
		p.Stop()

		var none *Progress
		require.NotPanics(t, func() {
			none.Advance()
			none.Stop()
		})
	})

	t.Run("shows the detail after the counter", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Running", 12, time.Millisecond)
		for range 5 {
			p.Advance()
		}
		p.SetDetail("waiting on backend")
		p.draw()

		assert.Contains(t, buf.String(), "Running 5/12, waiting on backend (")
	})

	t.Run("an empty detail removes it", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Running", 2, time.Millisecond)
		p.SetDetail("waiting on backend")
		p.SetDetail("")
		p.draw()

		assert.Contains(t, buf.String(), "Running 0/2 (")
		assert.NotContains(t, buf.String(), "waiting on")
	})

	t.Run("strips control sequences from the detail", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, false, "Running", 1, time.Millisecond)
		p.SetDetail("waiting on \x1b[31mevil\rname")
		p.draw()

		assert.Contains(t, buf.String(), "waiting on evilname")
	})

	t.Run("Print clears the line, prints the block and draws the line again", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Running", 3, time.Hour)
		defer p.Stop()
		p.draw()
		p.Advance()
		p.SetDetail("waiting on web")

		drawnBefore := buf.Len()
		calls := 0
		p.Print(func() {
			calls++
			buf.WriteString("block\n")
		})

		assert.Equal(t, 1, calls)
		printed := buf.String()[drawnBefore:]
		assert.True(t, strings.HasPrefix(printed, clearLine+"block\n"+clearLine), "the line must be removed before the block and drawn again after it, got %q", printed)
		assert.Contains(t, printed, "Running 1/3, waiting on web (")
	})

	t.Run("Print only runs the block while nothing is drawn", func(t *testing.T) {
		tests := []struct {
			name     string
			progress func(w *bytes.Buffer) *Progress
		}{
			{name: "nil", progress: func(*bytes.Buffer) *Progress { return nil }},
			{name: "disabled", progress: func(w *bytes.Buffer) *Progress {
				return newProgress(w, false, "Running", 2, time.Millisecond)
			}},
			{name: "before the first tick", progress: func(w *bytes.Buffer) *Progress {
				return newProgress(w, true, "Running", 2, time.Hour)
			}},
			{name: "after Stop", progress: func(w *bytes.Buffer) *Progress {
				p := newProgress(w, true, "Running", 2, time.Hour)
				p.Stop()
				return p
			}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				p := tt.progress(&buf)
				defer p.Stop()

				calls := 0
				p.Print(func() {
					calls++
					buf.WriteString("block\n")
				})

				assert.Equal(t, 1, calls)
				assert.Equal(t, "block\n", buf.String())
			})
		}
	})

	t.Run("a stopped line is not drawn again by Print", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Running", 2, time.Hour)
		p.draw()
		p.Stop()
		stopped := buf.String()
		require.True(t, strings.HasSuffix(stopped, clearLine), "Stop must clear the drawn line")

		p.Print(func() { buf.WriteString("block\n") })

		assert.Equal(t, stopped+"block\n", buf.String())
	})

	t.Run("Print never interleaves with a redraw", func(t *testing.T) {
		w := &lockedBuffer{}
		p := newProgress(w, true, "Running", 50, time.Microsecond)

		for i := range 50 {
			p.Print(func() {
				w.write(fmt.Sprintf("<block %d>", i))
				time.Sleep(50 * time.Microsecond)
				w.write(fmt.Sprintf("</block %d>", i))
			})
			p.Advance()
		}
		p.Stop()

		out := w.String()
		for i := range 50 {
			block := fmt.Sprintf("<block %d></block %d>", i, i)
			assert.Equal(t, 1, strings.Count(out, block), "block %d must be printed once and in one piece", i)
		}
	})

	t.Run("SetDetail and a nil progress", func(t *testing.T) {
		var none *Progress
		require.NotPanics(t, func() { none.SetDetail("waiting on api") })
	})

	t.Run("Advance is safe for concurrent use", func(t *testing.T) {
		var buf bytes.Buffer
		p := newProgress(&buf, true, "Pulling", 100, time.Millisecond)
		defer p.Stop()

		var wg sync.WaitGroup
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.Advance()
			}()
		}
		wg.Wait()
		p.Stop()
		buf.Reset()
		p.draw()

		assert.Contains(t, buf.String(), "Pulling 100/100")
	})
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) write(s string) {
	_, _ = b.Write([]byte(s))
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// drawnLine draws p once and returns the line as the terminal shows it.
func drawnLine(p *Progress, buf *bytes.Buffer) string {
	buf.Reset()
	p.draw()
	return strings.TrimPrefix(ansiSequence.ReplaceAllString(buf.String(), ""), "\r")
}

func TestProgressFitsTheTerminalWidth(t *testing.T) {
	const longPath = "services/payments/payment-gateway-integration-tests"

	tests := []struct {
		name     string
		width    int
		detail   string
		wantLine string
	}{
		{
			name:     "a detail that fits is shown in full",
			width:    80,
			detail:   "waiting on backend",
			wantLine: "⠋ Running 12/40, waiting on backend (0s)",
		},
		{
			name:     "a long detail keeps the end of the project path",
			width:    80,
			detail:   "waiting on " + longPath,
			wantLine: "⠋ Running 12/40, …g on services/payments/payment-gateway-integration-tests (0s)",
		},
		{
			name:     "an unknown width caps the detail",
			width:    0,
			detail:   "waiting on " + longPath,
			wantLine: "⠋ Running 12/40, …ments/payment-gateway-integration-tests (0s)",
		},
		{
			name:     "a terminal too narrow for the detail drops it",
			width:    22,
			detail:   "waiting on " + longPath,
			wantLine: "⠋ Running 12/40 (0s)",
		},
		{
			name:     "a terminal too narrow for the counter cuts the line",
			width:    12,
			detail:   "waiting on " + longPath,
			wantLine: "⠋ Running 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := newProgress(&buf, false, "Running", 40, time.Millisecond)
			p.width = func() int { return tt.width }
			for range 12 {
				p.Advance()
			}
			p.SetDetail(tt.detail)

			line := drawnLine(p, &buf)

			assert.Equal(t, tt.wantLine, line)
			if tt.width > 0 {
				assert.Less(t, utf8.RuneCountInString(line), tt.width, "the line must leave the last column free, or it wraps")
			}
		})
	}
}

func TestProgressRedrawAfterPrintFitsTheTerminalWidth(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, false, "Running", 3, time.Millisecond)
	p.width = func() int { return 30 }
	p.SetDetail("waiting on services/payments/payment-gateway-integration-tests")
	p.draw()

	buf.Reset()
	p.Print(func() { buf.WriteString("block\n") })

	redrawn := strings.TrimPrefix(ansiSequence.ReplaceAllString(strings.TrimPrefix(buf.String(), clearLine+"block\n"), ""), "\r")
	assert.Equal(t, "⠙ Running 0/3, …on-tests (0s)", redrawn)
}
