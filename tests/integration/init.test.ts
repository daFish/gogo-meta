import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { initCommand } from '../../src/commands/init.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../src/core/output.js', () => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
}));

describe('init command', () => {
  beforeEach(() => {
    vol.reset();
    vi.clearAllMocks();
    vi.spyOn(process, 'cwd').mockReturnValue('/project');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('creates .gogo file with default config', async () => {
    vol.fromJSON({ '/project': null });

    await initCommand();

    expect(vol.existsSync('/project/.gogo')).toBe(true);

    const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
    const config = JSON.parse(content);

    expect(config.projects).toEqual({});
    expect(config.ignore).toContain('.git');
    expect(config.ignore).toContain('node_modules');
  });

  it('throws error if .gogo already exists', async () => {
    vol.fromJSON({
      '/project/.gogo': '{"projects":{}}',
    });

    await expect(initCommand()).rejects.toThrow('already exists');
  });

  it('overwrites existing .gogo when force is true', async () => {
    vol.fromJSON({
      '/project/.gogo': '{"projects":{"old":"url"}}',
    });

    await initCommand({ force: true });

    const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
    const config = JSON.parse(content);

    expect(config.projects).toEqual({});
  });

  it('logs success message', async () => {
    vol.fromJSON({ '/project': null });
    const output = await import('../../src/core/output.js');

    await initCommand();

    expect(output.success).toHaveBeenCalledWith(expect.stringContaining('Created'));
  });

  it('logs warning when overwriting', async () => {
    vol.fromJSON({
      '/project/.gogo': '{}',
    });
    const output = await import('../../src/core/output.js');

    await initCommand({ force: true });

    expect(output.warning).toHaveBeenCalledWith(expect.stringContaining('Overwriting'));
  });
});
