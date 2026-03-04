import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { execCommand } from '../../src/commands/exec.js';
import { runCommand } from '../../src/commands/run.js';
import { importCommand } from '../../src/commands/project/import.js';
import { setOverlayFiles } from '../../src/core/config.js';

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
  dim: vi.fn(),
}));

describe('overlay config (-f flag)', () => {
  const mockExecute = vi.fn();

  beforeEach(async () => {
    vol.reset();
    vi.clearAllMocks();
    setOverlayFiles([]);

    vi.spyOn(process, 'cwd').mockReturnValue('/project');

    const executor = await import('../../src/core/executor.js');
    (executor.execute as ReturnType<typeof vi.fn>).mockImplementation(mockExecute);
  });

  afterEach(() => {
    setOverlayFiles([]);
    vi.restoreAllMocks();
  });

  describe('exec with overlay', () => {
    it('merges projects from overlay file', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'git@github.com:org/api.git' },
          ignore: [],
        }),
        '/project/.gogo.devops': JSON.stringify({
          projects: { infra: 'git@github.com:org/infra.git' },
          ignore: [],
        }),
      });

      setOverlayFiles(['.gogo.devops']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await execCommand('echo test');

      expect(mockExecute).toHaveBeenCalledTimes(2);
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/api' });
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/infra' });
    });

    it('accumulates projects from multiple overlay files', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
        }),
        '/project/.gogo.devops': JSON.stringify({
          projects: { infra: 'url2' },
          ignore: [],
        }),
        '/project/.gogo.extra': JSON.stringify({
          projects: { docs: 'url3' },
          ignore: [],
        }),
      });

      setOverlayFiles(['.gogo.devops', '.gogo.extra']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await execCommand('echo test');

      expect(mockExecute).toHaveBeenCalledTimes(3);
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/api' });
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/infra' });
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/docs' });
    });

    it('overlay project overrides primary on key conflict', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url-primary' },
          ignore: [],
        }),
        '/project/.gogo.override': JSON.stringify({
          projects: { api: 'url-overlay' },
          ignore: [],
        }),
      });

      setOverlayFiles(['.gogo.override']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await execCommand('echo test');

      expect(mockExecute).toHaveBeenCalledTimes(1);
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/api' });
    });

    it('throws error when overlay file does not exist', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url' },
          ignore: [],
        }),
      });

      setOverlayFiles(['.gogo.missing']);

      await expect(execCommand('echo test')).rejects.toThrow('Overlay config file not found');
    });
  });

  describe('run with overlay', () => {
    it('sees commands defined in overlay file', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
          commands: { build: 'npm run build' },
        }),
        '/project/.gogo.devops': JSON.stringify({
          projects: {},
          ignore: [],
          commands: { deploy: 'npm run deploy' },
        }),
      });

      setOverlayFiles(['.gogo.devops']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await runCommand('deploy');

      expect(mockExecute).toHaveBeenCalledTimes(1);
      expect(mockExecute).toHaveBeenCalledWith('npm run deploy', { cwd: '/project/api' });
    });

    it('overlay command overrides primary command', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
          commands: { build: 'npm run build' },
        }),
        '/project/.gogo.override': JSON.stringify({
          projects: {},
          ignore: [],
          commands: { build: 'yarn build' },
        }),
      });

      setOverlayFiles(['.gogo.override']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await runCommand('build');

      expect(mockExecute).toHaveBeenCalledTimes(1);
      expect(mockExecute).toHaveBeenCalledWith('yarn build', { cwd: '/project/api' });
    });
  });

  describe('YAML overlay', () => {
    it('supports YAML overlay files', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
        }),
        '/project/extra.yaml': 'projects:\n  web: url2\nignore: []\n',
      });

      setOverlayFiles(['extra.yaml']);
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      await execCommand('echo test');

      expect(mockExecute).toHaveBeenCalledTimes(2);
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/api' });
      expect(mockExecute).toHaveBeenCalledWith('echo test', { cwd: '/project/web' });
    });
  });

  describe('write commands with overlay', () => {
    it('import does not absorb overlay projects into primary config', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
        }),
        '/project/.gogo.devops': JSON.stringify({
          projects: { infra: 'url2' },
          ignore: [],
        }),
        '/project/.gitignore': '',
      });

      setOverlayFiles(['.gogo.devops']);
      mockExecute.mockResolvedValue({
        exitCode: 0,
        stdout: 'git@github.com:org/existing.git',
        stderr: '',
      });

      vol.mkdirSync('/project/existing', { recursive: true });
      vol.writeFileSync('/project/existing/.git', '');

      await importCommand('existing');

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const config = JSON.parse(content);

      // Primary config should have original + newly imported, but NOT overlay projects
      expect(config.projects.api).toBe('url1');
      expect(config.projects.existing).toBe('git@github.com:org/existing.git');
      expect(config.projects.infra).toBeUndefined();
    });

    it('import --no-clone does not absorb overlay projects', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { api: 'url1' },
          ignore: [],
        }),
        '/project/.gogo.devops': JSON.stringify({
          projects: { infra: 'url2' },
          ignore: [],
        }),
      });

      setOverlayFiles(['.gogo.devops']);

      await importCommand('web', 'git@github.com:org/web.git', { noClone: true });

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const config = JSON.parse(content);

      expect(config.projects.api).toBe('url1');
      expect(config.projects.web).toBe('git@github.com:org/web.git');
      expect(config.projects.infra).toBeUndefined();
    });
  });
});
