import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { createCommand } from '../../src/commands/project/create.js';
import { importCommand } from '../../src/commands/project/import.js';

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

describe('project commands', () => {
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

  describe('create command', () => {
    it('creates new project directory and initializes git', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          ignore: [],
        }),
        '/project/.gitignore': 'node_modules\n',
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await createCommand('libs/new-project', 'git@github.com:org/new.git');

      expect(vol.existsSync('/project/libs/new-project')).toBe(true);
      expect(mockExecute).toHaveBeenCalledWith('git init', expect.any(Object));
      expect(mockExecute).toHaveBeenCalledWith(
        expect.stringContaining('git remote add origin'),
        expect.any(Object)
      );
    });

    it('updates .gogo file with new project', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          ignore: [],
        }),
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await createCommand('api', 'git@github.com:org/api.git');

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const config = JSON.parse(content);

      expect(config.projects.api).toBe('git@github.com:org/api.git');
    });

    it('throws error if directory already exists', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
        '/project/existing/.git': '',
      });

      await expect(
        createCommand('existing', 'url')
      ).rejects.toThrow('already exists');
    });

    it('throws error when not in meta repository', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(
        createCommand('test', 'url')
      ).rejects.toThrow('Not in a gogo-meta repository');
    });

    it('adds project to .gitignore', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
        '/project/.gitignore': 'node_modules\n',
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await createCommand('api', 'url');

      const gitignore = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
      expect(gitignore).toContain('api');
    });
  });

  describe('import command', () => {
    it('clones repository when directory does not exist', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          ignore: [],
        }),
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await importCommand('libs/external', 'git@github.com:org/external.git');

      expect(mockExecute).toHaveBeenCalledWith(
        expect.stringContaining('git clone'),
        expect.any(Object)
      );
    });

    it('imports existing directory with remote', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
        '/project/existing/.git': '',
        '/project/.gitignore': '',
      });

      mockExecute.mockResolvedValue({
        exitCode: 0,
        stdout: 'git@github.com:org/existing.git',
        stderr: '',
      });

      await importCommand('existing');

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const config = JSON.parse(content);

      expect(config.projects.existing).toBe('git@github.com:org/existing.git');
    });

    it('throws error for existing directory without remote and no URL', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
        '/project/existing/.git': '',
      });

      mockExecute.mockResolvedValue({ exitCode: 1, stdout: '', stderr: '' });

      await expect(importCommand('existing')).rejects.toThrow('has no remote');
    });

    it('throws error for non-existent directory without URL', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
      });

      await expect(importCommand('nonexistent')).rejects.toThrow('URL is required');
    });

    it('updates .gogo file with imported project', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({ projects: {}, ignore: [] }),
      });

      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await importCommand('api', 'git@github.com:org/api.git');

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const config = JSON.parse(content);

      expect(config.projects.api).toBe('git@github.com:org/api.git');
    });
  });
});
