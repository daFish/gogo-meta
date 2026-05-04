//nolint:errcheck // Terminal output write errors are intentionally ignored.
package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Writer is the default output destination. Override in tests.
var Writer io.Writer = os.Stdout

// ErrWriter is the default error output destination. Override in tests.
var ErrWriter io.Writer = os.Stderr

var (
	greenFn    = color.New(color.FgGreen).SprintFunc()
	redFn      = color.New(color.FgRed).SprintFunc()
	yellowFn   = color.New(color.FgYellow).SprintFunc()
	blueFn     = color.New(color.FgBlue).SprintFunc()
	cyanFn     = color.New(color.FgCyan).SprintFunc()
	grayFn     = color.New(color.FgHiBlack).SprintFunc()
	dimFn      = color.New(color.Faint).SprintFunc()
	boldFn     = color.New(color.Bold).SprintFunc()
	boldCyanFn = color.New(color.Bold, color.FgCyan).SprintFunc()
)

// Symbols used in terminal output.
var (
	SuccessSymbol = greenFn("✓")
	ErrorSymbol   = redFn("✗")
	WarningSymbol = yellowFn("⚠")
	InfoSymbol    = blueFn("ℹ")
	ArrowSymbol   = cyanFn("→")
	BulletSymbol  = grayFn("•")
)

func Success(message string) {
	fmt.Fprintf(Writer, "%s %s\n", SuccessSymbol, message)
}

func Error(message string) {
	fmt.Fprintf(ErrWriter, "%s %s\n", ErrorSymbol, redFn(message))
}

func Warning(message string) {
	fmt.Fprintf(ErrWriter, "%s %s\n", WarningSymbol, yellowFn(message))
}

func Info(message string) {
	fmt.Fprintf(Writer, "%s %s\n", InfoSymbol, message)
}

func Header(directory string) {
	fmt.Fprintf(Writer, "\n%s %s\n", ArrowSymbol, boldCyanFn(directory))
}

func Dim(message string) {
	fmt.Fprintf(Writer, "%s\n", dimFn(message))
}

func Bold(message string) string {
	return boldFn(message)
}

func ProjectStatus(directory, status, message string) {
	var symbol any
	var colorFn func(a ...any) string

	if status == "success" {
		symbol = SuccessSymbol
		colorFn = greenFn
	} else {
		symbol = ErrorSymbol
		colorFn = redFn
	}

	suffix := ""
	if message != "" {
		suffix = " " + dimFn(message)
	}
	fmt.Fprintf(Writer, "%s %s%s\n", symbol, colorFn(directory), suffix)
}

func CommandOutput(stdout, stderr string) {
	if strings.TrimSpace(stdout) != "" {
		fmt.Fprintln(Writer, stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		fmt.Fprintln(ErrWriter, dimFn(stderr))
	}
}

// SummaryData holds the data for a summary output.
type SummaryData struct {
	Success        int
	Failed         int
	Total          int
	FailedProjects []string
}

func Summary(data SummaryData) {
	fmt.Fprintln(Writer)
	if data.Failed == 0 {
		fmt.Fprintf(Writer, "%s %s\n", SuccessSymbol, greenFn(fmt.Sprintf("All %d projects completed successfully", data.Total)))
	} else {
		fmt.Fprintf(Writer, "%s %s\n", WarningSymbol, yellowFn(fmt.Sprintf("%d/%d projects succeeded, %d failed", data.Success, data.Total, data.Failed)))

		if len(data.FailedProjects) > 0 {
			fmt.Fprintln(Writer)
			fmt.Fprintln(Writer, "Failed projects:")
			for _, project := range data.FailedProjects {
				fmt.Fprintf(Writer, "  %s %s\n", ErrorSymbol, redFn(project))
			}
		}
	}
}

func FormatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
}
