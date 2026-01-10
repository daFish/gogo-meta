import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import {
  readMetaConfig,
  writeMetaConfig,
  createDefaultConfig,
  addProject,
  removeProject,
  getProjectPaths,
  getProjectUrl,
  findFileUp,
  fileExists,
  ConfigError,
  META_FILE,
} from '../../../src/core/config.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

describe('config', () => {
  beforeEach(() => {
    vol.reset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('createDefaultConfig', () => {
    it('creates config with empty projects', () => {
      const config = createDefaultConfig();
      expect(config.projects).toEqual({});
    });

    it('includes default ignore patterns', () => {
      const config = createDefaultConfig();
      expect(config.ignore).toContain('.git');
      expect(config.ignore).toContain('node_modules');
    });
  });

  describe('addProject', () => {
    it('adds a new project to config', () => {
      const config = createDefaultConfig();
      const updated = addProject(config, 'libs/core', 'git@github.com:org/core.git');

      expect(updated.projects['libs/core']).toBe('git@github.com:org/core.git');
    });

    it('preserves existing projects', () => {
      const config = {
        projects: { existing: 'url1' },
        ignore: [],
      };
      const updated = addProject(config, 'new', 'url2');

      expect(updated.projects['existing']).toBe('url1');
      expect(updated.projects['new']).toBe('url2');
    });

    it('does not mutate original config', () => {
      const config = createDefaultConfig();
      addProject(config, 'test', 'url');

      expect(config.projects).toEqual({});
    });
  });

  describe('removeProject', () => {
    it('removes a project from config', () => {
      const config = {
        projects: { a: 'url1', b: 'url2' },
        ignore: [],
      };
      const updated = removeProject(config, 'a');

      expect(updated.projects['a']).toBeUndefined();
      expect(updated.projects['b']).toBe('url2');
    });

    it('handles removing non-existent project', () => {
      const config = createDefaultConfig();
      const updated = removeProject(config, 'nonexistent');

      expect(updated.projects).toEqual({});
    });
  });

  describe('getProjectPaths', () => {
    it('returns all project paths', () => {
      const config = {
        projects: { 'libs/a': 'url1', 'libs/b': 'url2' },
        ignore: [],
      };

      expect(getProjectPaths(config)).toEqual(['libs/a', 'libs/b']);
    });

    it('returns empty array for empty projects', () => {
      const config = createDefaultConfig();
      expect(getProjectPaths(config)).toEqual([]);
    });
  });

  describe('getProjectUrl', () => {
    it('returns url for existing project', () => {
      const config = {
        projects: { test: 'git@example.com:test.git' },
        ignore: [],
      };

      expect(getProjectUrl(config, 'test')).toBe('git@example.com:test.git');
    });

    it('returns undefined for non-existent project', () => {
      const config = createDefaultConfig();
      expect(getProjectUrl(config, 'nonexistent')).toBeUndefined();
    });
  });

  describe('fileExists', () => {
    it('returns true for existing file', async () => {
      vol.fromJSON({ '/test/file.txt': 'content' });

      expect(await fileExists('/test/file.txt')).toBe(true);
    });

    it('returns false for non-existent file', async () => {
      vol.fromJSON({});

      expect(await fileExists('/nonexistent')).toBe(false);
    });
  });

  describe('findFileUp', () => {
    it('finds file in current directory', async () => {
      vol.fromJSON({ '/project/.meta': '{}' });

      const result = await findFileUp('.meta', '/project');
      expect(result).toBe('/project/.meta');
    });

    it('finds file in parent directory', async () => {
      vol.fromJSON({
        '/project/.meta': '{}',
        '/project/sub/file.txt': '',
      });

      const result = await findFileUp('.meta', '/project/sub');
      expect(result).toBe('/project/.meta');
    });

    it('returns null when file not found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      const result = await findFileUp('.meta', '/project');
      expect(result).toBeNull();
    });
  });

  describe('readMetaConfig', () => {
    it('reads and parses valid .meta file', async () => {
      const metaContent = JSON.stringify({
        projects: { 'libs/core': 'git@github.com:org/core.git' },
        ignore: ['.git'],
      });
      vol.fromJSON({ '/project/.meta': metaContent });

      const config = await readMetaConfig('/project');

      expect(config.projects['libs/core']).toBe('git@github.com:org/core.git');
      expect(config.ignore).toContain('.git');
    });

    it('throws ConfigError when .meta file not found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('throws ConfigError for invalid JSON', async () => {
      vol.fromJSON({ '/project/.meta': 'not json' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('provides default ignore when not specified', async () => {
      vol.fromJSON({ '/project/.meta': '{"projects":{}}' });

      const config = await readMetaConfig('/project');
      expect(config.ignore).toContain('.git');
      expect(config.ignore).toContain('node_modules');
    });
  });

  describe('writeMetaConfig', () => {
    it('writes config to .meta file', async () => {
      vol.fromJSON({ '/project': null });

      const config = {
        projects: { test: 'url' },
        ignore: ['.git'],
      };

      await writeMetaConfig('/project', config);

      const content = vol.readFileSync('/project/.meta', 'utf-8') as string;
      const parsed = JSON.parse(content);

      expect(parsed.projects.test).toBe('url');
    });

    it('formats JSON with indentation', async () => {
      vol.fromJSON({ '/project': null });

      await writeMetaConfig('/project', createDefaultConfig());

      const content = vol.readFileSync('/project/.meta', 'utf-8') as string;
      expect(content).toContain('\n');
    });
  });
});
