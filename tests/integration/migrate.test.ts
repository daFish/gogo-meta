import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { migrateCommand } from '../../src/commands/migrate.js';

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

const mockExecute = vi.fn();

function withRemotes(remotes: Record<string, string>): void {
  mockExecute.mockImplementation(async (_cmd: string, opts: { cwd: string }) => {
    const url = remotes[opts.cwd];
    return url
      ? { exitCode: 0, stdout: url, stderr: '' }
      : { exitCode: 1, stdout: '', stderr: 'no such remote' };
  });
}

describe('migrate command', () => {
  beforeEach(async () => {
    vol.reset();
    vi.clearAllMocks();
    vi.spyOn(process, 'cwd').mockReturnValue('/project');

    const executor = await import('../../src/core/executor.js');
    (executor.execute as ReturnType<typeof vi.fn>).mockImplementation(mockExecute);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    process.exitCode = undefined;
  });

  it('throws when not in a gogo-meta repository', async () => {
    vol.fromJSON({ '/project/file.txt': '' });

    await expect(migrateCommand()).rejects.toThrow('Not in a gogo-meta repository');
  });

  it('reports already in sync when the working copy matches the config', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { api: 'git@github.com:org/api.git' } }),
      '/project/api/.git': '',
    });
    withRemotes({ '/project/api': 'git@github.com:org/api.git' });
    const output = await import('../../src/core/output.js');

    await migrateCommand();

    expect(output.success).toHaveBeenCalledWith(expect.stringContaining('already matches'));
    expect(vol.existsSync('/project/api')).toBe(true);
  });

  it('moves a repo from its current path to the configured path', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { 'packages/api': 'git@github.com:org/api.git' } }),
      '/project/lib/api/.git': '',
    });
    withRemotes({ '/project/lib/api': 'git@github.com:org/api.git' });
    const output = await import('../../src/core/output.js');

    await migrateCommand();

    expect(vol.existsSync('/project/packages/api/.git')).toBe(true);
    expect(vol.existsSync('/project/lib/api')).toBe(false);
    expect(output.projectStatus).toHaveBeenCalledWith(
      'packages/api',
      'success',
      expect.stringContaining('lib/api')
    );
  });

  it('removes a parent directory left empty by a move', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { 'packages/api': 'git@github.com:org/api.git' } }),
      '/project/lib/api/.git': '',
    });
    withRemotes({ '/project/lib/api': 'git@github.com:org/api.git' });

    await migrateCommand();

    expect(vol.existsSync('/project/lib')).toBe(false);
  });

  it('keeps a parent directory that still contains other repos after a move', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({
        projects: {
          'packages/api': 'git@github.com:org/api.git',
          'lib/web': 'git@github.com:org/web.git',
        },
      }),
      '/project/lib/api/.git': '',
      '/project/lib/web/.git': '',
    });
    withRemotes({
      '/project/lib/api': 'git@github.com:org/api.git',
      '/project/lib/web': 'git@github.com:org/web.git',
    });

    await migrateCommand();

    expect(vol.existsSync('/project/packages/api/.git')).toBe(true);
    expect(vol.existsSync('/project/lib/web/.git')).toBe(true);
    expect(vol.existsSync('/project/lib')).toBe(true);
  });

  it('updates .gitignore when a repo is moved', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { 'packages/api': 'git@github.com:org/api.git' } }),
      '/project/.gitignore': 'node_modules\nlib/api\n',
      '/project/lib/api/.git': '',
    });
    withRemotes({ '/project/lib/api': 'git@github.com:org/api.git' });

    await migrateCommand();

    const gitignore = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
    expect(gitignore).not.toMatch(/^lib\/api$/m);
    expect(gitignore).toMatch(/^packages\/api$/m);
  });

  it('does not move anything in dry-run mode', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { 'packages/api': 'git@github.com:org/api.git' } }),
      '/project/lib/api/.git': '',
    });
    withRemotes({ '/project/lib/api': 'git@github.com:org/api.git' });
    const output = await import('../../src/core/output.js');

    await migrateCommand({ dryRun: true });

    expect(vol.existsSync('/project/lib/api')).toBe(true);
    expect(vol.existsSync('/project/packages/api')).toBe(false);
    expect(output.info).toHaveBeenCalledWith(expect.stringContaining('packages/api'));
  });

  it('refuses to migrate when the target path is occupied by a different repository', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { api: 'git@github.com:org/api.git' } }),
      '/project/api/.git': '',
    });
    withRemotes({ '/project/api': 'git@github.com:org/other.git' });
    const output = await import('../../src/core/output.js');

    await expect(migrateCommand()).rejects.toThrow(/Migration aborted/);

    expect(output.projectStatus).toHaveBeenCalledWith(
      'api',
      'error',
      expect.stringContaining('occupied')
    );
  });

  it('warns and exits non-zero when a configured repo is not in the working copy', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { api: 'git@github.com:org/api.git' } }),
    });
    withRemotes({});
    const output = await import('../../src/core/output.js');

    await migrateCommand();

    expect(output.warning).toHaveBeenCalledWith(expect.stringContaining('gogo git update'));
    expect(process.exitCode).toBe(1);
  });
});
