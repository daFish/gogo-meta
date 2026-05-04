package executor

import (
	"bytes"
	"context"
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

// Executor is the interface for running shell commands.
type Executor interface {
	Execute(ctx context.Context, command string, opts Options) (*Result, error)
}

// ShellExecutor executes commands via /bin/sh.
type ShellExecutor struct{}

// NewShellExecutor creates a new ShellExecutor.
func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

// Execute runs a shell command asynchronously with timeout and process group management.
func (e *ShellExecutor) Execute(ctx context.Context, command string, opts Options) (*Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	// Set process group for clean cleanup on Unix.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	timedOut := ctx.Err() == context.DeadlineExceeded

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = 124
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

// ExecuteSync runs a command synchronously (blocking).
func ExecuteSync(command string, opts Options) *Result {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	timedOut := ctx.Err() == context.DeadlineExceeded

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = 124
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
	}
}

// StreamingOptions extends Options with streaming callbacks.
type StreamingOptions struct {
	Options
	OnStdout func(data string)
	OnStderr func(data string)
}

// ExecuteStreaming runs a command with streaming stdout/stderr callbacks.
func ExecuteStreaming(ctx context.Context, command string, opts StreamingOptions) (*Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = opts.Cwd
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return &Result{ExitCode: 1, Stderr: err.Error()}, nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return &Result{ExitCode: 1, Stderr: err.Error()}, nil
	}

	if err := cmd.Start(); err != nil {
		return &Result{ExitCode: 1, Stderr: err.Error()}, nil
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	done := make(chan struct{}, 2)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				stdoutBuf.WriteString(chunk)
				if opts.OnStdout != nil {
					opts.OnStdout(chunk)
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				stderrBuf.WriteString(chunk)
				if opts.OnStderr != nil {
					opts.OnStderr(chunk)
				}
			}
			if err != nil {
				break
			}
		}
		done <- struct{}{}
	}()

	<-done
	<-done

	waitErr := cmd.Wait()

	timedOut := ctx.Err() == context.DeadlineExceeded

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if timedOut {
			exitCode = 124
		} else {
			exitCode = 1
		}
	}

	if timedOut {
		exitCode = 124
	}

	return &Result{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdoutBuf.String()),
		Stderr:   strings.TrimSpace(stderrBuf.String()),
		TimedOut: timedOut,
	}, nil
}
