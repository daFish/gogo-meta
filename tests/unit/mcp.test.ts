import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { vol } from 'memfs';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { InMemoryTransport } from '@modelcontextprotocol/sdk/inMemory.js';
import { createServer } from '../../src/mcp.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../src/core/executor.js', () => ({
  execute: vi.fn(),
  executeSync: vi.fn(),
  executeStreaming: vi.fn(),
}));

vi.mock('../../src/core/output.js', () => ({
  header: vi.fn(),
  commandOutput: vi.fn(),
  warning: vi.fn(),
  summary: vi.fn(),
  info: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
  dim: vi.fn(),
  bold: vi.fn(),
  projectStatus: vi.fn(),
  formatDuration: vi.fn(),
  symbols: { success: '', error: '', warning: '', info: '', arrow: '', bullet: '' },
}));

const CONFIG = {
  projects: {
    api: 'git@github.com:org/api.git',
    web: 'git@github.com:org/web.git',
  },
  ignore: ['.git', 'node_modules'],
  commands: {
    build: 'npm run build',
    test: { cmd: 'npm test', parallel: true },
  },
};

async function setupClientServer() {
  const server = createServer();
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  await server.connect(serverTransport);
  const client = new Client({ name: 'test-client', version: '1.0.0' });
  await client.connect(clientTransport);
  return { client, server };
}

describe('MCP Server', () => {
  let mockExecute: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    vol.reset();
    vi.resetAllMocks();

    const executor = await import('../../src/core/executor.js');
    mockExecute = executor.execute as ReturnType<typeof vi.fn>;
    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

    vol.fromJSON({
      '/meta/.gogo': JSON.stringify(CONFIG),
      '/meta/api/.git/HEAD': '',
      '/meta/web/.git/HEAD': '',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('tool listing', () => {
    it('exposes all expected tools', async () => {
      const { client } = await setupClientServer();
      const { tools } = await client.listTools();
      const names = tools.map((t) => t.name).sort();

      expect(names).toEqual([
        'gogo_commands',
        'gogo_config',
        'gogo_exec',
        'gogo_git_branch',
        'gogo_git_checkout',
        'gogo_git_pull',
        'gogo_git_push',
        'gogo_git_status',
        'gogo_project_add',
        'gogo_project_remove',
        'gogo_projects',
        'gogo_run',
      ]);
    });
  });

  describe('gogo_config', () => {
    it('returns the config for a valid project', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_config', arguments: { cwd: '/meta' } });
      const parsed = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(parsed.metaDir).toBe('/meta');
      expect(parsed.projects).toEqual(CONFIG.projects);
    });

    it('returns error when no .gogo file found', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_config', arguments: { cwd: '/nonexistent' } });

      expect(result.isError).toBe(true);
      expect((result.content as Array<{ text: string }>)[0].text).toContain('No .gogo file found');
    });
  });

  describe('gogo_projects', () => {
    it('lists all projects with existence status', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_projects', arguments: { cwd: '/meta' } });
      const projects = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(projects).toHaveLength(2);
      expect(projects[0].path).toBe('api');
      expect(projects[0].url).toBe('git@github.com:org/api.git');
      expect(projects[0].exists).toBe(true);
      expect(projects[1].path).toBe('web');
    });

    it('reports non-existent project directories', async () => {
      vol.reset();
      vol.fromJSON({
        '/meta/.gogo': JSON.stringify({
          projects: { missing: 'git@github.com:org/missing.git' },
          ignore: [],
        }),
      });

      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_projects', arguments: { cwd: '/meta' } });
      const projects = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(projects[0].exists).toBe(false);
    });
  });

  describe('gogo_exec', () => {
    it('executes a command across all repos', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: 'ok', stderr: '' });

      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_exec',
        arguments: { command: 'echo hello', cwd: '/meta' },
      });
      const results = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(results).toHaveLength(2);
      expect(results[0].success).toBe(true);
      expect(results[0].stdout).toBe('ok');
      expect(mockExecute).toHaveBeenCalledTimes(2);
    });

    it('supports includeOnly filter', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_exec',
        arguments: { command: 'echo test', cwd: '/meta', includeOnly: 'api' },
      });
      const results = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(results).toHaveLength(1);
      expect(results[0].project).toBe('api');
    });

    it('supports excludeOnly filter', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_exec',
        arguments: { command: 'echo test', cwd: '/meta', excludeOnly: 'web' },
      });
      const results = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(results).toHaveLength(1);
      expect(results[0].project).toBe('api');
    });

    it('reports failures per project', async () => {
      mockExecute
        .mockResolvedValueOnce({ exitCode: 0, stdout: 'ok', stderr: '' })
        .mockResolvedValueOnce({ exitCode: 1, stdout: '', stderr: 'fail' });

      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_exec',
        arguments: { command: 'test', cwd: '/meta' },
      });
      const results = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(results[0].success).toBe(true);
      expect(results[1].success).toBe(false);
      expect(results[1].stderr).toBe('fail');
    });
  });

  describe('gogo_git_status', () => {
    it('runs git status across repos', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: 'On branch main', stderr: '' });

      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_git_status',
        arguments: { cwd: '/meta' },
      });
      const results = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(results).toHaveLength(2);
      expect(mockExecute).toHaveBeenCalledWith('git status', expect.objectContaining({ cwd: '/meta/api' }));
      expect(mockExecute).toHaveBeenCalledWith('git status', expect.objectContaining({ cwd: '/meta/web' }));
    });
  });

  describe('gogo_git_pull', () => {
    it('runs git pull across repos', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: 'Already up to date.', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_git_pull', arguments: { cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('git pull', expect.objectContaining({ cwd: '/meta/api' }));
      expect(mockExecute).toHaveBeenCalledWith('git pull', expect.objectContaining({ cwd: '/meta/web' }));
    });
  });

  describe('gogo_git_push', () => {
    it('runs git push across repos', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_git_push', arguments: { cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('git push', expect.objectContaining({ cwd: '/meta/api' }));
    });
  });

  describe('gogo_git_branch', () => {
    it('lists branches when no name given', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '* main\n  dev', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_git_branch', arguments: { cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('git branch', expect.objectContaining({ cwd: '/meta/api' }));
    });

    it('creates a branch when name given', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_git_branch', arguments: { name: 'feature-x', cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('git branch feature-x', expect.objectContaining({ cwd: '/meta/api' }));
    });
  });

  describe('gogo_git_checkout', () => {
    it('checks out a branch', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_git_checkout', arguments: { branch: 'dev', cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('git checkout dev', expect.objectContaining({ cwd: '/meta/api' }));
    });

    it('creates and checks out with -b flag', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({
        name: 'gogo_git_checkout',
        arguments: { branch: 'new-branch', create: true, cwd: '/meta' },
      });

      expect(mockExecute).toHaveBeenCalledWith(
        'git checkout -b new-branch',
        expect.objectContaining({ cwd: '/meta/api' }),
      );
    });
  });

  describe('gogo_commands', () => {
    it('lists predefined commands', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_commands', arguments: { cwd: '/meta' } });
      const commands = JSON.parse((result.content as Array<{ text: string }>)[0].text);

      expect(commands).toHaveLength(2);
      expect(commands[0].name).toBe('build');
      expect(commands[0].command.cmd).toBe('npm run build');
      expect(commands[1].name).toBe('test');
      expect(commands[1].command.parallel).toBe(true);
    });
  });

  describe('gogo_run', () => {
    it('runs a predefined command', async () => {
      mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });

      const { client } = await setupClientServer();
      await client.callTool({ name: 'gogo_run', arguments: { name: 'build', cwd: '/meta' } });

      expect(mockExecute).toHaveBeenCalledWith('npm run build', expect.objectContaining({ cwd: '/meta/api' }));
    });

    it('returns error for unknown command', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({ name: 'gogo_run', arguments: { name: 'nonexistent', cwd: '/meta' } });

      expect(result.isError).toBe(true);
      expect((result.content as Array<{ text: string }>)[0].text).toContain("Command 'nonexistent' not found");
      expect((result.content as Array<{ text: string }>)[0].text).toContain('build');
    });
  });

  describe('gogo_project_add', () => {
    it('adds a project to config', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_project_add',
        arguments: { path: 'new-svc', url: 'git@github.com:org/new-svc.git', cwd: '/meta' },
      });

      expect((result.content as Array<{ text: string }>)[0].text).toContain("Project 'new-svc' added");

      // Verify the config was written
      const { readFile } = await import('node:fs/promises');
      const written = JSON.parse(await readFile('/meta/.gogo', 'utf-8'));
      expect(written.projects['new-svc']).toBe('git@github.com:org/new-svc.git');
      expect(written.projects.api).toBe('git@github.com:org/api.git');
    });
  });

  describe('gogo_project_remove', () => {
    it('removes a project from config', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_project_remove',
        arguments: { path: 'api', cwd: '/meta' },
      });

      expect((result.content as Array<{ text: string }>)[0].text).toContain("Project 'api' removed");

      const { readFile } = await import('node:fs/promises');
      const written = JSON.parse(await readFile('/meta/.gogo', 'utf-8'));
      expect(written.projects.api).toBeUndefined();
      expect(written.projects.web).toBe('git@github.com:org/web.git');
    });

    it('returns error for nonexistent project', async () => {
      const { client } = await setupClientServer();
      const result = await client.callTool({
        name: 'gogo_project_remove',
        arguments: { path: 'nonexistent', cwd: '/meta' },
      });

      expect(result.isError).toBe(true);
      expect((result.content as Array<{ text: string }>)[0].text).toContain('not found');
    });
  });
});
