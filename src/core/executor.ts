import { spawn, execSync, type ExecSyncOptions } from 'node:child_process';
import type { ExecutorOptions, ExecutorResult } from '../types/index.js';

const DEFAULT_TIMEOUT = 300_000; // 5 minutes

export async function execute(command: string, options: ExecutorOptions): Promise<ExecutorResult> {
  const { cwd, env = process.env, timeout = DEFAULT_TIMEOUT, shell = true } = options;

  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let timedOut = false;

    const child = spawn(command, {
      cwd,
      env,
      shell,
      stdio: ['inherit', 'pipe', 'pipe'],
    });

    const timeoutId = setTimeout(() => {
      timedOut = true;
      child.kill('SIGTERM');
      setTimeout(() => {
        if (!child.killed) {
          child.kill('SIGKILL');
        }
      }, 5000);
    }, timeout);

    child.stdout?.on('data', (data: Buffer) => {
      stdout += data.toString();
    });

    child.stderr?.on('data', (data: Buffer) => {
      stderr += data.toString();
    });

    child.on('close', (code) => {
      clearTimeout(timeoutId);
      resolve({
        exitCode: code ?? (timedOut ? 124 : 1),
        stdout: stdout.trim(),
        stderr: stderr.trim(),
        timedOut,
      });
    });

    child.on('error', (err) => {
      clearTimeout(timeoutId);
      resolve({
        exitCode: 1,
        stdout: '',
        stderr: err.message,
        timedOut: false,
      });
    });
  });
}

function getShellPath(shell: boolean | undefined): string | undefined {
  if (shell === false || shell === undefined) return undefined;
  return process.platform === 'win32' ? process.env['ComSpec'] || 'cmd.exe' : '/bin/sh';
}

export function executeSync(command: string, options: ExecutorOptions): ExecutorResult {
  const { cwd, env = process.env, timeout = DEFAULT_TIMEOUT, shell = true } = options;

  try {
    const execOptions: ExecSyncOptions = {
      cwd,
      env,
      shell: getShellPath(shell),
      timeout,
      encoding: 'utf-8',
      stdio: ['inherit', 'pipe', 'pipe'],
    };
    const result = execSync(command, execOptions) as string;

    return {
      exitCode: 0,
      stdout: result.trim(),
      stderr: '',
      timedOut: false,
    };
  } catch (error: unknown) {
    const err = error as {
      status?: number;
      stdout?: string;
      stderr?: string;
      killed?: boolean;
      signal?: string;
    };

    return {
      exitCode: err.status ?? 1,
      stdout: err.stdout?.trim() ?? '',
      stderr: err.stderr?.trim() ?? '',
      timedOut: err.killed === true || err.signal === 'SIGTERM',
    };
  }
}

export async function executeStreaming(
  command: string,
  options: ExecutorOptions & {
    onStdout?: (data: string) => void;
    onStderr?: (data: string) => void;
  }
): Promise<ExecutorResult> {
  const { cwd, env = process.env, timeout = DEFAULT_TIMEOUT, shell = true, onStdout, onStderr } = options;

  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let timedOut = false;

    const child = spawn(command, {
      cwd,
      env,
      shell,
      stdio: ['inherit', 'pipe', 'pipe'],
    });

    const timeoutId = setTimeout(() => {
      timedOut = true;
      child.kill('SIGTERM');
    }, timeout);

    child.stdout?.on('data', (data: Buffer) => {
      const str = data.toString();
      stdout += str;
      onStdout?.(str);
    });

    child.stderr?.on('data', (data: Buffer) => {
      const str = data.toString();
      stderr += str;
      onStderr?.(str);
    });

    child.on('close', (code) => {
      clearTimeout(timeoutId);
      resolve({
        exitCode: code ?? (timedOut ? 124 : 1),
        stdout: stdout.trim(),
        stderr: stderr.trim(),
        timedOut,
      });
    });

    child.on('error', (err) => {
      clearTimeout(timeoutId);
      resolve({
        exitCode: 1,
        stdout: '',
        stderr: err.message,
        timedOut: false,
      });
    });
  });
}
