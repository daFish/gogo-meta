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
  addToGitignore,
  ConfigError,
  normalizeCommand,
  getCommand,
  listCommands,
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

  describe('addToGitignore', () => {
    it('creates .gitignore if it does not exist', async () => {
      vol.fromJSON({ '/project': null });

      const added = await addToGitignore('/project', 'api');

      expect(added).toBe(true);
      expect(vol.existsSync('/project/.gitignore')).toBe(true);
      const content = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
      expect(content).toBe('api\n');
    });

    it('appends entry to existing .gitignore', async () => {
      vol.fromJSON({ '/project/.gitignore': 'node_modules\n' });

      const added = await addToGitignore('/project', 'api');

      expect(added).toBe(true);
      const content = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
      expect(content).toBe('node_modules\napi\n');
    });

    it('returns false and skips if entry already exists', async () => {
      vol.fromJSON({ '/project/.gitignore': 'node_modules\napi\n' });

      const added = await addToGitignore('/project', 'api');

      expect(added).toBe(false);
      const content = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
      expect(content).toBe('node_modules\napi\n');
    });

    it('handles .gitignore without trailing newline', async () => {
      vol.fromJSON({ '/project/.gitignore': 'node_modules' });

      const added = await addToGitignore('/project', 'api');

      expect(added).toBe(true);
      const content = vol.readFileSync('/project/.gitignore', 'utf-8') as string;
      expect(content).toBe('node_modules\napi\n');
    });

    it('handles entry with surrounding whitespace in file', async () => {
      vol.fromJSON({ '/project/.gitignore': '  api  \nnode_modules\n' });

      const added = await addToGitignore('/project', 'api');

      expect(added).toBe(false);
    });
  });

  describe('findFileUp', () => {
    it('finds file in current directory', async () => {
      vol.fromJSON({ '/project/.gogo': '{}' });

      const result = await findFileUp('.gogo', '/project');
      expect(result).toBe('/project/.gogo');
    });

    it('finds file in parent directory', async () => {
      vol.fromJSON({
        '/project/.gogo': '{}',
        '/project/sub/file.txt': '',
      });

      const result = await findFileUp('.gogo', '/project/sub');
      expect(result).toBe('/project/.gogo');
    });

    it('returns null when file not found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      const result = await findFileUp('.gogo', '/project');
      expect(result).toBeNull();
    });
  });

  describe('readMetaConfig', () => {
    it('reads and parses valid .gogo file', async () => {
      const metaContent = JSON.stringify({
        projects: { 'libs/core': 'git@github.com:org/core.git' },
        ignore: ['.git'],
      });
      vol.fromJSON({ '/project/.gogo': metaContent });

      const config = await readMetaConfig('/project');

      expect(config.projects['libs/core']).toBe('git@github.com:org/core.git');
      expect(config.ignore).toContain('.git');
    });

    it('throws ConfigError when .gogo file not found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('throws ConfigError for invalid JSON', async () => {
      vol.fromJSON({ '/project/.gogo': 'not json' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('provides default ignore when not specified', async () => {
      vol.fromJSON({ '/project/.gogo': '{"projects":{}}' });

      const config = await readMetaConfig('/project');
      expect(config.ignore).toContain('.git');
      expect(config.ignore).toContain('node_modules');
    });
  });

  describe('writeMetaConfig', () => {
    it('writes config to .gogo file', async () => {
      vol.fromJSON({ '/project': null });

      const config = {
        projects: { test: 'url' },
        ignore: ['.git'],
      };

      await writeMetaConfig('/project', config);

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const parsed = JSON.parse(content);

      expect(parsed.projects.test).toBe('url');
    });

    it('formats JSON with indentation', async () => {
      vol.fromJSON({ '/project': null });

      await writeMetaConfig('/project', createDefaultConfig());

      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      expect(content).toContain('\n');
    });
  });

  describe('normalizeCommand', () => {
    it('normalizes string command to object', () => {
      const result = normalizeCommand('npm run build');
      expect(result).toEqual({ cmd: 'npm run build' });
    });

    it('passes through object command', () => {
      const input = { cmd: 'npm test', parallel: true, description: 'Run tests' };
      const result = normalizeCommand(input);
      expect(result).toEqual(input);
    });

    it('preserves all object properties', () => {
      const input = {
        cmd: 'npm test',
        parallel: true,
        concurrency: 4,
        description: 'Run tests',
        includeOnly: ['api', 'web'],
        excludeOnly: ['docs'],
        includePattern: '^libs/',
        excludePattern: 'test$',
      };
      const result = normalizeCommand(input);
      expect(result).toEqual(input);
    });
  });

  describe('getCommand', () => {
    it('returns undefined for non-existent command', () => {
      const config = { projects: {}, ignore: [] };
      expect(getCommand(config, 'build')).toBeUndefined();
    });

    it('returns undefined when commands not defined', () => {
      const config = { projects: {}, ignore: [], commands: undefined };
      expect(getCommand(config, 'build')).toBeUndefined();
    });

    it('returns normalized string command', () => {
      const config = {
        projects: {},
        ignore: [],
        commands: { build: 'npm run build' },
      };
      expect(getCommand(config, 'build')).toEqual({ cmd: 'npm run build' });
    });

    it('returns normalized object command', () => {
      const config = {
        projects: {},
        ignore: [],
        commands: {
          test: { cmd: 'npm test', parallel: true, includeOnly: ['api'] },
        },
      };
      expect(getCommand(config, 'test')).toEqual({
        cmd: 'npm test',
        parallel: true,
        includeOnly: ['api'],
      });
    });
  });

  describe('listCommands', () => {
    it('returns empty array when no commands', () => {
      const config = { projects: {}, ignore: [] };
      expect(listCommands(config)).toEqual([]);
    });

    it('returns empty array when commands is undefined', () => {
      const config = { projects: {}, ignore: [], commands: undefined };
      expect(listCommands(config)).toEqual([]);
    });

    it('returns normalized command list', () => {
      const config = {
        projects: {},
        ignore: [],
        commands: {
          build: 'npm run build',
          test: { cmd: 'npm test', description: 'Run all tests' },
        },
      };
      expect(listCommands(config)).toEqual([
        { name: 'build', command: { cmd: 'npm run build' } },
        { name: 'test', command: { cmd: 'npm test', description: 'Run all tests' } },
      ]);
    });
  });

  describe('MetaConfigSchema with commands', () => {
    it('parses config with string command', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          commands: { build: 'npm run build' },
        }),
      });

      const config = await readMetaConfig('/project');
      expect(config.commands?.build).toBe('npm run build');
    });

    it('parses config with object command', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          commands: {
            test: {
              cmd: 'npm test',
              parallel: true,
              concurrency: 2,
              description: 'Run tests',
              includeOnly: ['api', 'web'],
            },
          },
        }),
      });

      const config = await readMetaConfig('/project');
      expect(config.commands?.test).toEqual({
        cmd: 'npm test',
        parallel: true,
        concurrency: 2,
        description: 'Run tests',
        includeOnly: ['api', 'web'],
      });
    });

    it('parses config with mixed commands', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          commands: {
            build: 'npm run build',
            test: { cmd: 'npm test', parallel: true },
          },
        }),
      });

      const config = await readMetaConfig('/project');
      expect(config.commands?.build).toBe('npm run build');
      expect(config.commands?.test).toEqual({ cmd: 'npm test', parallel: true });
    });

    it('rejects invalid command structure', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          commands: { build: { invalid: true } },
        }),
      });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('rejects command object without cmd field', async () => {
      vol.fromJSON({
        '/project/.gogo': JSON.stringify({
          projects: {},
          commands: { build: { parallel: true } },
        }),
      });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });
  });
});
