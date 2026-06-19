package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const DefaultTimeout = 5 * time.Minute

// Options configures command execution.
type Options struct {
	Cwd     string
	Env     []string
	Timeout time.Duration
}

// Result holds the output of a command execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	TimedOut bool
}

// Executor runs commands. Execute uses /bin/sh -c (for user shell commands);
// ExecuteArgs runs an argv directly with no shell (for built-in commands).
type Executor interface {
	Execute(ctx context.Context, command string, opts Options) (*Result, error)
	ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error)
}

// ShellExecutor executes commands via /bin/sh or directly.
type ShellExecutor struct{}

// NewShellExecutor creates a new ShellExecutor.
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

// Execute runs a command string via /bin/sh -c. Use only where running an
// arbitrary shell command is the contract (gogo exec / run).
func (e *ShellExecutor) Execute(ctx context.Context, command string, opts Options) (*Result, error) {
	return e.run(ctx, "/bin/sh", []string{"-c", command}, opts)
}

// ExecuteArgs runs name with args directly — no shell, so args cannot be
// interpreted as shell syntax.
func (e *ShellExecutor) ExecuteArgs(ctx context.Context, name string, args []string, opts Options) (*Result, error) {
	return e.run(ctx, name, args, opts)
}

func (e *ShellExecutor) run(ctx context.Context, name string, args []string, opts Options) (*Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	// Run each command in its own process group and, on context cancellation
	// (Ctrl-C or timeout), kill the whole group so no children are orphaned.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	timedOut := ctx.Err() == context.DeadlineExceeded

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	if timedOut {
		exitCode = 124
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		TimedOut: timedOut,
	}, nil
}
