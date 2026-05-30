import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { validateCommand } from '../../src/commands/validate.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../src/core/output.js', () => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  projectStatus: vi.fn(),
}));

describe('validate command', () => {
  beforeEach(() => {
    vol.reset();
    vi.clearAllMocks();
    vi.spyOn(process, 'cwd').mockReturnValue('/project');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('warns when no config files found', async () => {
    vol.fromJSON({ '/project': null });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.warning).toHaveBeenCalledWith(expect.stringContaining('No config files'));
  });

  it('validates a valid .gogo JSON file', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: { 'lib/foo': 'git@github.com:org/foo.git' } }),
      '/project/lib/foo/.git': '',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
  });

  it('validates a valid .gogo.yaml file', async () => {
    vol.fromJSON({
      '/project/.gogo.yaml': 'projects:\n  lib/foo: git@github.com:org/foo.git\n',
      '/project/lib/foo/.git': '',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yaml', 'success');
  });

  it('validates a valid .gogo.yml file', async () => {
    vol.fromJSON({
      '/project/.gogo.yml': 'projects:\n  lib/foo: git@github.com:org/foo.git\n',
      '/project/lib/foo/.git': '',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yml', 'success');
  });

  it('reports invalid JSON in .gogo', async () => {
    vol.fromJSON({
      '/project/.gogo': '{not valid json',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'error', 'Invalid JSON');
  });

  it('reports invalid YAML in .gogo.yaml', async () => {
    vol.fromJSON({
      '/project/.gogo.yaml': ':\n  - :\n  invalid: [',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yaml', 'error', 'Invalid YAML');
  });

  it('reports invalid structure in .gogo', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: 'not-an-object' }),
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'error', expect.stringContaining('Invalid structure'));
  });

  it('validates multiple files at once', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/.gogo.yaml': 'projects:\n  lib/bar: git@github.com:org/bar.git\n',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledTimes(2);
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yaml', 'success');
  });

  it('reports mix of valid and invalid files', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/.gogo.devops': '{broken',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.devops', 'error', 'Invalid JSON');
  });

  it('validates overlay config files like .gogo.devops.yaml', async () => {
    vol.fromJSON({
      '/project/.gogo.devops.yaml': 'projects:\n  infra/deploy: git@github.com:org/deploy.git\n',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.devops.yaml', 'success');
  });

  it('reports invalid overlay config files', async () => {
    vol.fromJSON({
      '/project/.gogo.devops': '{broken',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.devops', 'error', 'Invalid JSON');
  });

  it('validates all config files including overlays', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/.gogo.devops.yaml': 'projects:\n  infra/cd: git@github.com:org/cd.git\n',
      '/project/.gogo.staging': JSON.stringify({ projects: { 'staging/app': 'git@github.com:org/app.git' } }),
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledTimes(3);
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.devops.yaml', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.staging', 'success');
  });

  it('ignores non-config files', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/package.json': '{}',
      '/project/README.md': 'hello',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledTimes(1);
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
  });

  describe('working copy validation', () => {
    it('fails when a configured project directory is missing', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { 'lib/foo': 'git@github.com:org/foo.git' },
        }),
      });
      const output = await import('../../src/core/output.js');

      await expect(validateCommand()).rejects.toThrow('Validation failed');

      expect(output.projectStatus).toHaveBeenCalledWith(
        'lib/foo',
        'error',
        expect.stringContaining('missing')
      );
    });

    it('suggests how to fix a missing project directory', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { 'lib/foo': 'git@github.com:org/foo.git' },
        }),
      });
      const output = await import('../../src/core/output.js');

      await expect(validateCommand()).rejects.toThrow('Validation failed');

      const call = (output.projectStatus as ReturnType<typeof vi.fn>).mock.calls.find(
        ([dir]) => dir === 'lib/foo'
      );
      expect(call?.[2]).toMatch(/gogo migrate/);
      expect(call?.[2]).toMatch(/gogo git update/);
    });

    it('passes when all configured project directories exist', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: { 'lib/foo': 'git@github.com:org/foo.git' },
        }),
        '/project/lib/foo/.git': '',
      });
      const output = await import('../../src/core/output.js');

      await expect(validateCommand()).resolves.toBeUndefined();

      expect(output.projectStatus).not.toHaveBeenCalledWith(
        'lib/foo',
        'error',
        expect.anything()
      );
    });

    it('validates the working copy for a .gogo.yaml config', async () => {
      vol.fromJSON({
        '/project/.gogo.yaml': 'projects:\n  lib/foo: git@github.com:org/foo.git\n',
      });
      const output = await import('../../src/core/output.js');

      await expect(validateCommand()).rejects.toThrow('Validation failed');

      expect(output.projectStatus).toHaveBeenCalledWith(
        'lib/foo',
        'error',
        expect.stringContaining('missing')
      );
    });

    it('reports each missing project while keeping present ones quiet', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {
            'lib/present': 'git@github.com:org/present.git',
            'lib/missing': 'git@github.com:org/missing.git',
          },
        }),
        '/project/lib/present/.git': '',
      });
      const output = await import('../../src/core/output.js');

      await expect(validateCommand()).rejects.toThrow('Validation failed');

      expect(output.projectStatus).toHaveBeenCalledWith(
        'lib/missing',
        'error',
        expect.stringContaining('missing')
      );
      expect(output.projectStatus).not.toHaveBeenCalledWith(
        'lib/present',
        'error',
        expect.anything()
      );
    });
  });
});
