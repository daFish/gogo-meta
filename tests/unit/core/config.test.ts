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
  findMetaFileUp,
  fileExists,
  addToGitignore,
  ConfigError,
  normalizeCommand,
  getCommand,
  listCommands,
  detectFormat,
  filenameForFormat,
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

  describe('detectFormat', () => {
    it('returns json for .gogo file', () => {
      expect(detectFormat('/project/.gogo')).toBe('json');
    });

    it('returns yaml for .gogo.yaml file', () => {
      expect(detectFormat('/project/.gogo.yaml')).toBe('yaml');
    });

    it('returns yaml for .gogo.yml file', () => {
      expect(detectFormat('/project/.gogo.yml')).toBe('yaml');
    });

    it('returns json for unknown extensions', () => {
      expect(detectFormat('/project/.gogo.toml')).toBe('json');
    });
  });

  describe('filenameForFormat', () => {
    it('returns .gogo for json format', () => {
      expect(filenameForFormat('json')).toBe('.gogo');
    });

    it('returns .gogo.yaml for yaml format', () => {
      expect(filenameForFormat('yaml')).toBe('.gogo.yaml');
    });
  });

  describe('findMetaFileUp', () => {
    it('finds .gogo file', async () => {
      vol.fromJSON({ '/project/.gogo': '{}' });

      const result = await findMetaFileUp('/project');
      expect(result).toBe('/project/.gogo');
    });

    it('finds .gogo.yaml file', async () => {
      vol.fromJSON({ '/project/.gogo.yaml': 'projects: {}' });

      const result = await findMetaFileUp('/project');
      expect(result).toBe('/project/.gogo.yaml');
    });

    it('finds .gogo.yml file', async () => {
      vol.fromJSON({ '/project/.gogo.yml': 'projects: {}' });

      const result = await findMetaFileUp('/project');
      expect(result).toBe('/project/.gogo.yml');
    });

    it('prefers .gogo over .gogo.yaml in same directory', async () => {
      vol.fromJSON({
        '/project/.gogo': '{"projects":{}}',
        '/project/.gogo.yaml': 'projects: {}',
      });

      const result = await findMetaFileUp('/project');
      expect(result).toBe('/project/.gogo');
    });

    it('prefers .gogo.yaml over .gogo.yml in same directory', async () => {
      vol.fromJSON({
        '/project/.gogo.yaml': 'projects: {}',
        '/project/.gogo.yml': 'projects: {}',
      });

      const result = await findMetaFileUp('/project');
      expect(result).toBe('/project/.gogo.yaml');
    });

    it('finds config in parent directory', async () => {
      vol.fromJSON({
        '/project/.gogo.yaml': 'projects: {}',
        '/project/sub/file.txt': '',
      });

      const result = await findMetaFileUp('/project/sub');
      expect(result).toBe('/project/.gogo.yaml');
    });

    it('prefers child directory match over parent', async () => {
      vol.fromJSON({
        '/parent/.gogo': '{"projects":{}}',
        '/parent/child/.gogo.yaml': 'projects: {}',
      });

      const result = await findMetaFileUp('/parent/child');
      expect(result).toBe('/parent/child/.gogo.yaml');
    });

    it('returns null when no config file found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      const result = await findMetaFileUp('/project');
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

      const { config, format, metaDir } = await readMetaConfig('/project');

      expect(config.projects['libs/core']).toBe('git@github.com:org/core.git');
      expect(config.ignore).toContain('.git');
      expect(format).toBe('json');
      expect(metaDir).toBe('/project');
    });

    it('throws ConfigError when config file not found', async () => {
      vol.fromJSON({ '/project/file.txt': '' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('throws ConfigError for invalid JSON', async () => {
      vol.fromJSON({ '/project/.gogo': 'not json' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('provides default ignore when not specified', async () => {
      vol.fromJSON({ '/project/.gogo': '{"projects":{}}' });

      const { config } = await readMetaConfig('/project');
      expect(config.ignore).toContain('.git');
      expect(config.ignore).toContain('node_modules');
    });
  });

  describe('readMetaConfig with YAML', () => {
    it('reads and parses valid .gogo.yaml file', async () => {
      const yamlContent = 'projects:\n  libs/core: "git@github.com:org/core.git"\nignore:\n  - .git\n';
      vol.fromJSON({ '/project/.gogo.yaml': yamlContent });

      const { config, format, metaDir } = await readMetaConfig('/project');

      expect(format).toBe('yaml');
      expect(metaDir).toBe('/project');
      expect(config.projects['libs/core']).toBe('git@github.com:org/core.git');
      expect(config.ignore).toContain('.git');
    });

    it('reads and parses valid .gogo.yml file', async () => {
      const yamlContent = 'projects:\n  api: "git@github.com:org/api.git"\nignore:\n  - .git\n';
      vol.fromJSON({ '/project/.gogo.yml': yamlContent });

      const { config, format } = await readMetaConfig('/project');

      expect(format).toBe('yaml');
      expect(config.projects['api']).toBe('git@github.com:org/api.git');
    });

    it('throws ConfigError for invalid YAML', async () => {
      vol.fromJSON({ '/project/.gogo.yaml': '{ invalid yaml: [' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('throws ConfigError for YAML with invalid structure', async () => {
      vol.fromJSON({ '/project/.gogo.yaml': 'projects: "not-an-object"' });

      await expect(readMetaConfig('/project')).rejects.toThrow(ConfigError);
    });

    it('provides default ignore when not specified in YAML', async () => {
      vol.fromJSON({ '/project/.gogo.yaml': 'projects: {}' });

      const { config } = await readMetaConfig('/project');
      expect(config.ignore).toContain('.git');
      expect(config.ignore).toContain('node_modules');
    });

    it('parses YAML with commands', async () => {
      const yamlContent = [
        'projects: {}',
        'commands:',
        '  build: npm run build',
        '  test:',
        '    cmd: npm test',
        '    parallel: true',
      ].join('\n');
      vol.fromJSON({ '/project/.gogo.yaml': yamlContent });

      const { config } = await readMetaConfig('/project');
      expect(config.commands?.build).toBe('npm run build');
      expect(config.commands?.test).toEqual({ cmd: 'npm test', parallel: true });
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

    it('defaults to JSON format', async () => {
      vol.fromJSON({ '/project': null });

      await writeMetaConfig('/project', createDefaultConfig());

      expect(vol.existsSync('/project/.gogo')).toBe(true);
      expect(vol.existsSync('/project/.gogo.yaml')).toBe(false);
    });
  });

  describe('writeMetaConfig with YAML', () => {
    it('writes config as YAML when format is yaml', async () => {
      vol.fromJSON({ '/project': null });

      const config = { projects: { test: 'url' }, ignore: ['.git'] };
      await writeMetaConfig('/project', config, 'yaml');

      expect(vol.existsSync('/project/.gogo.yaml')).toBe(true);
      expect(vol.existsSync('/project/.gogo')).toBe(false);
      const content = vol.readFileSync('/project/.gogo.yaml', 'utf-8') as string;
      expect(content).toContain('projects:');
      expect(content).toContain('test: url');
    });

    it('writes config as JSON when format is json', async () => {
      vol.fromJSON({ '/project': null });

      const config = { projects: { test: 'url' }, ignore: ['.git'] };
      await writeMetaConfig('/project', config, 'json');

      expect(vol.existsSync('/project/.gogo')).toBe(true);
      const content = vol.readFileSync('/project/.gogo', 'utf-8') as string;
      const parsed = JSON.parse(content);
      expect(parsed.projects.test).toBe('url');
    });

    it('round-trips YAML config correctly', async () => {
      const yamlContent = 'projects:\n  api: "git@github.com:org/api.git"\nignore:\n  - .git\n';
      vol.fromJSON({ '/project/.gogo.yaml': yamlContent });

      const { config, format } = await readMetaConfig('/project');

      vol.reset();
      vol.fromJSON({ '/project': null });

      await writeMetaConfig('/project', config, format);

      const { config: reread, format: rereadFormat } = await readMetaConfig('/project');
      expect(reread).toEqual(config);
      expect(rereadFormat).toBe('yaml');
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

      const { config } = await readMetaConfig('/project');
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

      const { config } = await readMetaConfig('/project');
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

      const { config } = await readMetaConfig('/project');
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
