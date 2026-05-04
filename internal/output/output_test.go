package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Disable color in tests for predictable output.
	color.NoColor = true
}

func captureOutput(fn func()) (stdout, stderr string) {
	var outBuf, errBuf bytes.Buffer
	origWriter := Writer
	origErrWriter := ErrWriter
	Writer = &outBuf
	ErrWriter = &errBuf
	defer func() {
		Writer = origWriter
		ErrWriter = origErrWriter
	}()
	fn()
	return outBuf.String(), errBuf.String()
}

func TestSymbols(t *testing.T) {
	assert.Contains(t, SuccessSymbol, "✓")
	assert.Contains(t, ErrorSymbol, "✗")
	assert.Contains(t, WarningSymbol, "⚠")
	assert.Contains(t, InfoSymbol, "ℹ")
	assert.Contains(t, ArrowSymbol, "→")
	assert.Contains(t, BulletSymbol, "•")
}

func TestSuccess(t *testing.T) {
	out, _ := captureOutput(func() { Success("Test message") })
	assert.Contains(t, out, "Test message")
	assert.Contains(t, out, "✓")
}

func TestError(t *testing.T) {
	_, errOut := captureOutput(func() { Error("Error message") })
	assert.Contains(t, errOut, "Error message")
	assert.Contains(t, errOut, "✗")
}

func TestWarning(t *testing.T) {
	_, errOut := captureOutput(func() { Warning("Warning message") })
	assert.Contains(t, errOut, "Warning message")
	assert.Contains(t, errOut, "⚠")
}

func TestInfo(t *testing.T) {
	out, _ := captureOutput(func() { Info("Info message") })
	assert.Contains(t, out, "Info message")
	assert.Contains(t, out, "ℹ")
}

func TestHeader(t *testing.T) {
	out, _ := captureOutput(func() { Header("libs/core") })
	assert.Contains(t, out, "libs/core")
	assert.Contains(t, out, "→")
}

func TestProjectStatus(t *testing.T) {
	t.Run("success status", func(t *testing.T) {
		out, _ := captureOutput(func() { ProjectStatus("api", "success", "") })
		assert.Contains(t, out, "api")
		assert.Contains(t, out, "✓")
	})

	t.Run("error status", func(t *testing.T) {
		out, _ := captureOutput(func() { ProjectStatus("api", "error", "failed") })
		assert.Contains(t, out, "api")
		assert.Contains(t, out, "✗")
	})

	t.Run("includes optional message", func(t *testing.T) {
		out, _ := captureOutput(func() { ProjectStatus("api", "success", "cloned") })
		assert.Contains(t, out, "cloned")
	})
}

func TestSummary(t *testing.T) {
	t.Run("all success", func(t *testing.T) {
		out, _ := captureOutput(func() {
			Summary(SummaryData{Success: 3, Failed: 0, Total: 3})
		})
		assert.Contains(t, out, "3")
		assert.Contains(t, out, "successfully")
	})

	t.Run("partial success", func(t *testing.T) {
		out, _ := captureOutput(func() {
			Summary(SummaryData{Success: 2, Failed: 1, Total: 3})
		})
		assert.Contains(t, out, "2/3")
		assert.Contains(t, out, "1 failed")
	})

	t.Run("lists failed projects", func(t *testing.T) {
		out, _ := captureOutput(func() {
			Summary(SummaryData{Success: 1, Failed: 2, Total: 3, FailedProjects: []string{"api", "libs/auth"}})
		})
		assert.Contains(t, out, "api")
		assert.Contains(t, out, "libs/auth")
	})

	t.Run("does not list failed projects when all succeed", func(t *testing.T) {
		out, _ := captureOutput(func() {
			Summary(SummaryData{Success: 2, Failed: 0, Total: 2, FailedProjects: []string{}})
		})
		assert.False(t, strings.Contains(out, "Failed"))
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 1500 * time.Millisecond, "1.5s"},
		{"exactly 1 second", 1000 * time.Millisecond, "1.0s"},
		{"zero", 0, "0ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatDuration(tt.duration))
		})
	}
}

func TestCommandOutput(t *testing.T) {
	t.Run("prints stdout", func(t *testing.T) {
		out, _ := captureOutput(func() { CommandOutput("hello world", "") })
		assert.Contains(t, out, "hello world")
	})

	t.Run("prints stderr", func(t *testing.T) {
		_, errOut := captureOutput(func() { CommandOutput("", "error output") })
		assert.Contains(t, errOut, "error output")
	})

	t.Run("skips empty stdout", func(t *testing.T) {
		out, _ := captureOutput(func() { CommandOutput("   ", "err") })
		assert.Empty(t, out)
	})

	t.Run("skips empty stderr", func(t *testing.T) {
		_, errOut := captureOutput(func() { CommandOutput("out", "   ") })
		assert.Empty(t, errOut)
	})
}

func TestDim(t *testing.T) {
	out, _ := captureOutput(func() { Dim("dimmed text") })
	assert.Contains(t, out, "dimmed text")
}

func TestBold(t *testing.T) {
	result := Bold("bold text")
	assert.Contains(t, result, "bold text")
}
