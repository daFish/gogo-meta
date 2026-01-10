import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { runCommand } from '../../src/commands/run.js';

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
  dim: vi.fn(),
}));

describe('run command', () => {
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

  it('executes string command across all projects', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1', web: 'url2' },
        commands: { build: 'npm run build' },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await runCommand('build');

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).toHaveBeenCalledWith('npm run build', { cwd: '/project/api' });
    expect(mockExecute).toHaveBeenCalledWith('npm run build', { cwd: '/project/web' });
  });

  it('executes object command with config options', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1', web: 'url2', docs: 'url3' },
        commands: {
          test: {
            cmd: 'npm test',
            includeOnly: ['api', 'web'],
          },
        },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await runCommand('test');

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).not.toHaveBeenCalledWith(expect.anything(), { cwd: '/project/docs' });
  });

  it('CLI options override config options', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1', web: 'url2', docs: 'url3' },
        commands: {
          test: {
            cmd: 'npm test',
            includeOnly: ['api', 'web'],
          },
        },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await runCommand('test', { includeOnly: 'docs' });

    expect(mockExecute).toHaveBeenCalledTimes(1);
    expect(mockExecute).toHaveBeenCalledWith('npm test', { cwd: '/project/docs' });
  });

  it('uses config parallel option', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1', web: 'url2' },
        commands: {
          build: {
            cmd: 'npm run build',
            parallel: true,
          },
        },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await runCommand('build');

    expect(mockExecute).toHaveBeenCalledTimes(2);
  });

  it('throws error for unknown command', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
        commands: { build: 'npm build' },
      }),
    });

    await expect(runCommand('nonexistent')).rejects.toThrow('Unknown command: "nonexistent"');
  });

  it('throws error for unknown command with helpful message', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
        commands: { build: 'npm build', test: 'npm test' },
      }),
    });

    await expect(runCommand('nonexistent')).rejects.toThrow('Available commands: build, test');
  });

  it('throws error when no commands defined', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
      }),
    });

    await expect(runCommand('build')).rejects.toThrow('No commands are defined');
  });

  it('lists commands with --list flag', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
        commands: { build: 'npm run build' },
      }),
    });

    const output = await import('../../src/core/output.js');

    await runCommand(undefined, { list: true });

    expect(output.info).toHaveBeenCalledWith('Available commands:');
    expect(mockExecute).not.toHaveBeenCalled();
  });

  it('lists commands when no name provided', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
        commands: { build: 'npm run build' },
      }),
    });

    const output = await import('../../src/core/output.js');

    await runCommand(undefined);

    expect(output.info).toHaveBeenCalledWith('Available commands:');
    expect(mockExecute).not.toHaveBeenCalled();
  });

  it('shows helpful message when no commands defined with --list', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
      }),
    });

    const output = await import('../../src/core/output.js');

    await runCommand(undefined, { list: true });

    expect(output.info).toHaveBeenCalledWith('No commands defined in .gogo file');
  });

  it('sets exit code on failure', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url' },
        commands: { build: 'npm build' },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 1, stdout: '', stderr: 'error' });

    await runCommand('build');

    expect(process.exitCode).toBe(1);
  });

  it('throws error when not in meta repository', async () => {
    vol.fromJSON({ '/project/file.txt': '' });

    await expect(runCommand('build')).rejects.toThrow('Not in a gogo-meta repository');
  });

  it('applies excludeOnly from config', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1', web: 'url2', docs: 'url3' },
        commands: {
          build: {
            cmd: 'npm run build',
            excludeOnly: ['docs'],
          },
        },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    await runCommand('build');

    expect(mockExecute).toHaveBeenCalledTimes(2);
    expect(mockExecute).not.toHaveBeenCalledWith(expect.anything(), { cwd: '/project/docs' });
  });

  it('displays command info before execution', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: { api: 'url1' },
        commands: { build: 'npm run build' },
      }),
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });
    const output = await import('../../src/core/output.js');

    await runCommand('build');

    expect(output.info).toHaveBeenCalledWith(expect.stringContaining('Running "build"'));
  });
});
