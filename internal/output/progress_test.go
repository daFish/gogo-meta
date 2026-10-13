package output

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

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
