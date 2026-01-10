import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { vol } from 'memfs';
import { loop, hasFailures, getExitCode, type LoopContext } from '../../../src/core/loop.js';
import type { MetaConfig, ExecutorResult } from '../../../src/types/index.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../../src/core/executor.js', () => ({
  execute: vi.fn(),
}));

vi.mock('../../../src/core/output.js', () => ({
  header: vi.fn(),
  commandOutput: vi.fn(),
  warning: vi.fn(),
  summary: vi.fn(),
  info: vi.fn(),
}));

describe('loop', () => {
  const mockExecute = vi.fn();

  beforeEach(async () => {
    vol.reset();
    vi.resetAllMocks();

    const executor = await import('../../../src/core/executor.js');
    (executor.execute as ReturnType<typeof vi.fn>).mockImplementation(mockExecute);

    vol.fromJSON({
      '/meta/.meta': JSON.stringify({ projects: {}, ignore: [] }),
      '/meta/api/.git': '',
      '/meta/web/.git': '',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  const createContext = (projects: Record<string, string> = {}): LoopContext => ({
    config: {
      projects,
      ignore: [],
    },
    metaDir: '/meta',
  });

  describe('loop execution', () => {
    it('executes command in each project directory', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const context = createContext({
        api: 'git@github.com:org/api.git',
        web: 'git@github.com:org/web.git',
      });

      await loop('echo test', context, { suppressOutput: true });

      expect(mockExecute).toHaveBeenCalledTimes(2);
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/meta/api' });
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/meta/web' });
    });

    it('returns results for each project', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: 'output', stderr: '' });

      const context = createContext({
        api: 'url1',
        web: 'url2',
      });

      const results = await loop('test', context, { suppressOutput: true });

      expect(results).toHaveLength(2);
      expect(results[0]?.directory).toBe('api');
      expect(results[1]?.directory).toBe('web');
    });

    it('tracks success/failure correctly', async () => {
      mockExecute
        .mockResolvedValueOnce({ exitCode: 0, stdout: '', stderr: '' })
        .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: 'error' });

      const context = createContext({
        api: 'url1',
        web: 'url2',
      });

      const results = await loop('test', context, { suppressOutput: true });

      expect(results[0]?.success).toBe(true);
      expect(results[1]?.success).toBe(false);
    });

    it('records execution duration', async () => {
      mockExecute.mockImplementation(
        () => new Promise((r) => setTimeout(() => r({ exitCode: 0, stdout: '', stderr: '' }), 50))
      );

      const context = createContext({ api: 'url' });

      const results = await loop('test', context, { suppressOutput: true });

      expect(results[0]?.duration).toBeGreaterThanOrEqual(40);
    });
  });

  describe('filtering', () => {
    it('applies includeOnly filter', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const context = createContext({
        api: 'url1',
        web: 'url2',
        docs: 'url3',
      });

      await loop('test', context, {
        includeOnly: ['api', 'web'],
        suppressOutput: true,
      });

      expect(mockExecute).toHaveBeenCalledTimes(2);
    });

    it('applies excludeOnly filter', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const context = createContext({
        api: 'url1',
        web: 'url2',
        docs: 'url3',
      });

      await loop('test', context, {
        excludeOnly: ['docs'],
        suppressOutput: true,
      });

      expect(mockExecute).toHaveBeenCalledTimes(2);
    });

    it('warns and returns empty when no projects match filters', async () => {
      const output = await import('../../../src/core/output.js');

      const context = createContext({
        api: 'url1',
      });

      const results = await loop('test', context, {
        includeOnly: ['nonexistent'],
        suppressOutput: true,
      });

      expect(results).toHaveLength(0);
      expect(output.warning).toHaveBeenCalledWith('No projects match the specified filters');
    });
  });

  describe('parallel execution', () => {
    it('executes in parallel when parallel option is true', async () => {
      const executionOrder: string[] = [];

      mockExecute.mockImplementation(async (cmd: string, opts: { cwd: string }) => {
        const dir = opts.cwd.split('/').pop();
        executionOrder.push(`start-${dir}`);
        await new Promise((r) => setTimeout(r, 10));
        executionOrder.push(`end-${dir}`);
        return { exitCode: 0, stdout: '', stderr: '' };
      });

      const context = createContext({
        a: 'url1',
        b: 'url2',
        c: 'url3',
      });

      await loop('test', context, {
        parallel: true,
        suppressOutput: true,
      });

      const startCount = executionOrder.filter((e) => e.startsWith('start')).length;
      const firstEnd = executionOrder.findIndex((e) => e.startsWith('end'));

      expect(startCount).toBe(3);
      expect(firstEnd).toBeGreaterThan(1);
    });
  });

  describe('function commands', () => {
    it('accepts function as command', async () => {
      const commandFn = vi.fn().mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const context = createContext({ api: 'url' });

      await loop(commandFn, context, { suppressOutput: true });

      expect(commandFn).toHaveBeenCalledWith('/meta/api', 'api');
    });
  });

  describe('hasFailures', () => {
    it('returns true when any result failed', () => {
      const results = [
        { directory: 'a', result: { exitCode: 0, stdout: '', stderr: '' }, success: true, duration: 0 },
        { directory: 'b', result: { exitCode: 1, stdout: '', stderr: '' }, success: false, duration: 0 },
      ];

      expect(hasFailures(results)).toBe(true);
    });

    it('returns false when all results succeeded', () => {
      const results = [
        { directory: 'a', result: { exitCode: 0, stdout: '', stderr: '' }, success: true, duration: 0 },
        { directory: 'b', result: { exitCode: 0, stdout: '', stderr: '' }, success: true, duration: 0 },
      ];

      expect(hasFailures(results)).toBe(false);
    });

    it('returns false for empty results', () => {
      expect(hasFailures([])).toBe(false);
    });
  });

  describe('getExitCode', () => {
    it('returns 0 when all succeeded', () => {
      const results = [
        { directory: 'a', result: { exitCode: 0, stdout: '', stderr: '' }, success: true, duration: 0 },
      ];

      expect(getExitCode(results)).toBe(0);
    });

    it('returns 1 when any failed', () => {
      const results = [
        { directory: 'a', result: { exitCode: 0, stdout: '', stderr: '' }, success: true, duration: 0 },
        { directory: 'b', result: { exitCode: 1, stdout: '', stderr: '' }, success: false, duration: 0 },
      ];

      expect(getExitCode(results)).toBe(1);
    });
  });
});
