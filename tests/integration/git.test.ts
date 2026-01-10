import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { updateCommand } from '../../src/commands/git/update.js';
import { statusCommand } from '../../src/commands/git/status.js';

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
  projectStatus: vi.fn(),
  bold: vi.fn((s: string) => s),
}));

describe('git commands', () => {
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

  describe('update command', () => {
    it('clones missing repositories', async () => {
      vol.fromJSON({
        '/project/.meta': JSON.stringify({
          projects: {
            api: 'git@github.com:org/api.git',
            web: 'git@github.com:org/web.git',
          },
          ignore: [],
        }),
        '/project/api/.git': '',
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await updateCommand();

      expect(mockExecute).toHaveBeenCalledTimes(1);
      expect(mockExecute).toHaveBeenCalledWith(
        expect.stringContaining('git clone'),
        expect.objectContaining({ cwd: '/project' })
      );
    });

    it('reports all cloned when no missing repos', async () => {
      vol.fromJSON({
        '/project/.meta': JSON.stringify({
          projects: { api: 'url' },
          ignore: [],
        }),
        '/project/api/.git': '',
      });

      const output = await import('../../src/core/output.js');

      await updateCommand();

      expect(output.success).toHaveBeenCalledWith(
        expect.stringContaining('already cloned')
      );
    });

    it('throws error when not in meta repository', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(updateCommand()).rejects.toThrow('Not in a gogo-meta repository');
    });

    it('respects filters', async () => {
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

      await updateCommand({ includeOnly: 'api,web' });

      expect(mockExecute).toHaveBeenCalledTimes(2);
    });
  });

  describe('status command', () => {
    it('runs git status in all projects', async () => {
      vol.fromJSON({
        '/project/.meta': JSON.stringify({
          projects: {
            api: 'url1',
            web: 'url2',
          },
          ignore: [],
        }),
      });

      mockExecute.mockResolvedValue({
        exitCode: 0,
        stdout: '## main',
        stderr: '',
      });

      await statusCommand();

      expect(mockExecute).toHaveBeenCalledTimes(2);
      expect(mockExecute).toHaveBeenCalledWith(
        'git status --short --branch',
        expect.objectContaining({ cwd: '/project/api' })
      );
    });

    it('throws error when not in meta repository', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(statusCommand()).rejects.toThrow('Not in a gogo-meta repository');
    });
  });
});
