import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { execCommand } from '../../src/commands/exec.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../src/core/executor.js', () => ({
  execute: vi.fn(),
}));

vi.mock('../../src/core/output.js', () => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  header: vi.fn(),
  commandOutput: vi.fn(),
  summary: vi.fn(),
  bold: vi.fn((s: string) => s),
}));

describe('exec command', () => {
  const mockExecute = vi.fn();

  beforeEach(async () => {
    vol.reset();
    vi.clearAllMocks();

    vi.spyOn(process, 'cwd').mockReturnValue('/project');

    const executor = await import('../../src/core/executor.js');
    (executor.execute as ReturnType<typeof vi.fn>).mockImplementation(mockExecute);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('executes command across all projects', async () => {
    vol.fromJSON({
      '/project/.meta': JSON.stringify({
        projects: {
          api: 'git@github.com:org/api.git',
          web: 'git@github.com:org/web.git',
        },
        ignore: [],
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: 'output', stderr: '' });

    await execCommand('echo test');

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/api' });
    expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/web' });
  });

  it('throws error when not in a meta repository', async () => {
    vol.fromJSON({
      '/project/file.txt': '',
    });

    await expect(execCommand('echo test')).rejects.toThrow('Not in a gogo-meta repository');
  });

  it('respects includeOnly filter', async () => {
    vol.fromJSON({
      '/project/.meta': JSON.stringify({
        projects: {
          api: 'url1',
          web: 'url2',
          docs: 'url3',
        },
        ignore: [],
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await execCommand('echo test', { includeOnly: 'api,web' });

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).not.toHaveBeenCalledWith(expect.anything(), { cwd: '/project/docs' });
  });

  it('respects excludeOnly filter', async () => {
    vol.fromJSON({
      '/project/.meta': JSON.stringify({
        projects: {
          api: 'url1',
          web: 'url2',
          docs: 'url3',
        },
        ignore: [],
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await execCommand('echo test', { excludeOnly: 'docs' });

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).not.toHaveBeenCalledWith(expect.anything(), { cwd: '/project/docs' });
  });

  it('sets exit code on failure', async () => {
    vol.fromJSON({
      '/project/.meta': JSON.stringify({
        projects: { api: 'url' },
        ignore: [],
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 1, stdout: '', stderr: 'error' });

    await execCommand('failing-command');

    expect(process.exitCode).toBe(1);
  });

  it('logs info message with command', async () => {
    vol.fromJSON({
      '/project/.meta': JSON.stringify({
        projects: { api: 'url' },
        ignore: [],
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });
    const output = await import('../../src/core/output.js');

    await execCommand('echo hello');

    expect(output.info).toHaveBeenCalledWith(expect.stringContaining('echo hello'));
  });
});
