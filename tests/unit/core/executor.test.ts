import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { execute } from '../../../src/core/executor.js';

describe('executor', () => {
  describe('execute', () => {
    it('executes a simple command successfully', async () => {
      const result = await execute('echo "hello"', { cwd: process.cwd() });

      expect(result.exitCode).toBe(0);
      expect(result.stdout).toBe('hello');
      expect(result.stderr).toBe('');
      expect(result.timedOut).toBe(false);
    });

    it('captures exit code on failure', async () => {
      const result = await execute('exit 42', { cwd: process.cwd() });

      expect(result.exitCode).toBe(42);
    });

    it('captures stderr', async () => {
      const result = await execute('echo "error" >&2', { cwd: process.cwd() });

      expect(result.stderr).toBe('error');
    });

    it('captures both stdout and stderr', async () => {
      const result = await execute('echo "out" && echo "err" >&2', { cwd: process.cwd() });

      expect(result.stdout).toBe('out');
      expect(result.stderr).toBe('err');
    });

    it('uses specified working directory', async () => {
      const result = await execute('pwd', { cwd: '/tmp' });

      expect(result.stdout).toMatch(/tmp/);
    });

    it('passes environment variables', async () => {
      const result = await execute('echo $TEST_VAR', {
        cwd: process.cwd(),
        env: { ...process.env, TEST_VAR: 'test_value' },
      });

      expect(result.stdout).toBe('test_value');
    });

    it('handles command not found', async () => {
      const result = await execute('nonexistentcommand12345', { cwd: process.cwd() });

      expect(result.exitCode).not.toBe(0);
    });

    it('handles timeout', async () => {
      const result = await execute('sleep 10', {
        cwd: process.cwd(),
        timeout: 100,
      });

      expect(result.timedOut).toBe(true);
      expect(result.exitCode).not.toBe(0);
    }, 10000);

    it('handles multiline output', async () => {
      const result = await execute('echo "line1"; echo "line2"; echo "line3"', {
        cwd: process.cwd(),
      });

      expect(result.stdout).toContain('line1');
      expect(result.stdout).toContain('line2');
      expect(result.stdout).toContain('line3');
    });

    it('handles empty output', async () => {
      const result = await execute('true', { cwd: process.cwd() });

      expect(result.exitCode).toBe(0);
      expect(result.stdout).toBe('');
    });

    it('handles special characters in output', async () => {
      const result = await execute('echo "hello\tworld"', { cwd: process.cwd() });

      expect(result.stdout).toContain('hello');
      expect(result.stdout).toContain('world');
    });
  });
});
