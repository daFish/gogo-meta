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
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
  });

  it('validates a valid .gogo.yaml file', async () => {
    vol.fromJSON({
      '/project/.gogo.yaml': 'projects:\n  lib/foo: git@github.com:org/foo.git\n',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yaml', 'success');
  });

  it('validates a valid .gogo.yml file', async () => {
    vol.fromJSON({
      '/project/.gogo.yml': 'projects:\n  lib/foo: git@github.com:org/foo.git\n',
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yml', 'success');
  });

  it('validates a valid .looprc file', async () => {
    vol.fromJSON({
      '/project/.looprc': JSON.stringify({ ignore: ['docs'] }),
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledWith('.looprc', 'success');
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

  it('reports invalid JSON in .looprc', async () => {
    vol.fromJSON({
      '/project/.looprc': 'not json',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.looprc', 'error', 'Invalid JSON');
  });

  it('validates multiple files at once', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/.gogo.yaml': 'projects:\n  lib/bar: git@github.com:org/bar.git\n',
      '/project/.looprc': JSON.stringify({ ignore: [] }),
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledTimes(3);
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.yaml', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.looprc', 'success');
  });

  it('reports mix of valid and invalid files', async () => {
    vol.fromJSON({
      '/project/.gogo': JSON.stringify({ projects: {} }),
      '/project/.looprc': 'broken',
    });
    const output = await import('../../src/core/output.js');

    await expect(validateCommand()).rejects.toThrow('Validation failed');

    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.looprc', 'error', 'Invalid JSON');
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
      '/project/.looprc': JSON.stringify({ ignore: [] }),
    });
    const output = await import('../../src/core/output.js');

    await validateCommand();

    expect(output.projectStatus).toHaveBeenCalledTimes(4);
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.devops.yaml', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.gogo.staging', 'success');
    expect(output.projectStatus).toHaveBeenCalledWith('.looprc', 'success');
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
});
