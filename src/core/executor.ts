import { spawn, execSync } from 'node:child_process';
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

export function executeSync(command: string, options: ExecutorOptions): ExecutorResult {
  const { cwd, env = process.env, timeout = DEFAULT_TIMEOUT, shell = true } = options;

  try {
    const result = execSync(command, {
      cwd,
      env,
      shell,
      timeout,
      encoding: 'utf-8',
      stdio: ['inherit', 'pipe', 'pipe'],
    });

    return {
      exitCode: 0,
      stdout: typeof result === 'string' ? result.trim() : '',
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
