import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { findGitRepos } from '../../../src/core/discover.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

describe('findGitRepos', () => {
  beforeEach(() => {
    vol.reset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns an empty list when there are no git repositories', async () => {
    vol.fromJSON({
      '/project/.gogo': '{}',
      '/project/docs/readme.md': 'hi',
    });

    expect(await findGitRepos('/project')).toEqual([]);
  });

  it('finds a git repository directly under the root', async () => {
    vol.fromJSON({
      '/project/api/.git': '',
    });

    expect(await findGitRepos('/project')).toEqual(['api']);
  });

  it('finds nested git repositories and reports paths relative to the root', async () => {
    vol.fromJSON({
      '/project/api/.git': '',
      '/project/libs/shared/.git': '',
    });

    expect(await findGitRepos('/project')).toEqual(['api', 'libs/shared']);
  });

  it('does not report the root itself even when it is a git repository', async () => {
    vol.fromJSON({
      '/project/.git': '',
      '/project/api/.git': '',
    });

    expect(await findGitRepos('/project')).toEqual(['api']);
  });

  it('does not recurse into a discovered repository', async () => {
    vol.fromJSON({
      '/project/api/.git': '',
      '/project/api/vendor/nested/.git': '',
    });

    expect(await findGitRepos('/project')).toEqual(['api']);
  });

  it('skips ignored directory names', async () => {
    vol.fromJSON({
      '/project/api/.git': '',
      '/project/node_modules/pkg/.git': '',
    });

    expect(await findGitRepos('/project', ['node_modules'])).toEqual(['api']);
  });
});
