#!/usr/bin/env node

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { z } from 'zod';
import { join } from 'node:path';
import {
  readMetaConfig,
  getMetaDir,
  getProjectPaths,
  getProjectUrl,
  listCommands,
  getCommand,
  addProject,
  removeProject,
  writeMetaConfig,
  fileExists,
} from './core/config.js';
import { loop } from './core/loop.js';
import type { LoopOptions, LoopResult } from './types/index.js';

async function resolveContext(cwd?: string) {
  const workDir = cwd ?? process.cwd();
  const metaDir = await getMetaDir(workDir);
  if (!metaDir) {
    throw new Error(`No .gogo file found. Run 'gogo init' to create one.`);
  }
  const config = await readMetaConfig(metaDir);
  return { config, metaDir };
}

function buildLoopOptions(params: {
  includeOnly?: string | undefined;
  excludeOnly?: string | undefined;
  parallel?: boolean | undefined;
  concurrency?: number | undefined;
}): LoopOptions {
  const opts: LoopOptions = { suppressOutput: true };
  if (params.includeOnly) {
    opts.includeOnly = params.includeOnly.split(',').map((s) => s.trim()).filter(Boolean);
  }
  if (params.excludeOnly) {
    opts.excludeOnly = params.excludeOnly.split(',').map((s) => s.trim()).filter(Boolean);
  }
  if (params.parallel) opts.parallel = true;
  if (params.concurrency) opts.concurrency = params.concurrency;
  return opts;
}

function formatLoopResults(results: LoopResult[]): string {
  return JSON.stringify(
    results.map((r) => ({
      project: r.directory,
      success: r.success,
      exitCode: r.result.exitCode,
      stdout: r.result.stdout,
      stderr: r.result.stderr,
      duration: r.duration,
    })),
    null,
    2,
  );
}

function errorResponse(error: unknown) {
  return {
    content: [{ type: 'text' as const, text: `Error: ${error instanceof Error ? error.message : String(error)}` }],
    isError: true,
  };
}

export function createServer(): McpServer {
  const server = new McpServer({
    name: 'gogo-meta',
    version: '1.0.0',
  });

  server.tool(
    'gogo_config',
    'Read the .gogo multi-repo configuration file. Returns the full config including projects, ignore patterns, and predefined commands.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
    },
    async ({ cwd }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        return {
          content: [{ type: 'text' as const, text: JSON.stringify({ metaDir, ...config }, null, 2) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_projects',
    'List all projects in the gogo-meta configuration with their paths, git URLs, and whether the directory exists on disk.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
    },
    async ({ cwd }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const paths = getProjectPaths(config);
        const projects = await Promise.all(
          paths.map(async (p) => ({
            path: p,
            url: getProjectUrl(config, p),
            exists: await fileExists(join(metaDir, p)),
          })),
        );
        return {
          content: [{ type: 'text' as const, text: JSON.stringify(projects, null, 2) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_exec',
    'Execute a shell command across all (or filtered) child repositories. Returns structured results with stdout, stderr, exit code, and duration per project.',
    {
      command: z.string().describe('The shell command to execute in each repository'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
      parallel: z.boolean().optional().describe('Run in parallel (default: false)'),
      concurrency: z.number().optional().describe('Max concurrent executions when parallel (default: 4)'),
    },
    async ({ command, cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const results = await loop(command, { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_git_status',
    'Get git status for all (or filtered) child repositories. Shows working tree changes, staged files, and branch info.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
    },
    async ({ cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const results = await loop('git status', { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_git_pull',
    'Pull latest changes from remote for all (or filtered) child repositories.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
      parallel: z.boolean().optional().describe('Run in parallel (default: false)'),
    },
    async ({ cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const results = await loop('git pull', { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_git_push',
    'Push local commits to remote for all (or filtered) child repositories.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
      parallel: z.boolean().optional().describe('Run in parallel (default: false)'),
    },
    async ({ cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const results = await loop('git push', { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_git_branch',
    'List or create branches across all (or filtered) child repositories.',
    {
      name: z.string().optional().describe('Branch name to create (omit to list branches)'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
    },
    async ({ name, cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const command = name ? `git branch ${name}` : 'git branch';
        const results = await loop(command, { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_git_checkout',
    'Checkout a branch across all (or filtered) child repositories.',
    {
      branch: z.string().describe('Branch name to checkout'),
      create: z.boolean().optional().describe('Create the branch if it does not exist (-b flag)'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
    },
    async ({ branch, create, cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const options = buildLoopOptions(filterParams);
        const command = create ? `git checkout -b ${branch}` : `git checkout ${branch}`;
        const results = await loop(command, { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_commands',
    'List all predefined commands from the .gogo configuration file.',
    {
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
    },
    async ({ cwd }) => {
      try {
        const { config } = await resolveContext(cwd);
        const commands = listCommands(config);
        return {
          content: [{ type: 'text' as const, text: JSON.stringify(commands, null, 2) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_run',
    'Run a predefined command from the .gogo configuration across all (or filtered) child repositories.',
    {
      name: z.string().describe('Name of the predefined command to run'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
      includeOnly: z.string().optional().describe('Comma-separated list of project names to include'),
      excludeOnly: z.string().optional().describe('Comma-separated list of project names to exclude'),
    },
    async ({ name, cwd, ...filterParams }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const resolved = getCommand(config, name);
        if (!resolved) {
          const available = listCommands(config)
            .map((c) => c.name)
            .join(', ');
          return {
            content: [
              {
                type: 'text' as const,
                text: `Command '${name}' not found. Available commands: ${available || '(none)'}`,
              },
            ],
            isError: true,
          };
        }
        const options = buildLoopOptions({
          ...filterParams,
          parallel: resolved.parallel,
          concurrency: resolved.concurrency,
        });
        if (resolved.includeOnly) options.includeOnly = resolved.includeOnly;
        if (resolved.excludeOnly) options.excludeOnly = resolved.excludeOnly;
        const results = await loop(resolved.cmd, { config, metaDir }, options);
        return {
          content: [{ type: 'text' as const, text: formatLoopResults(results) }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_project_add',
    'Add a new project (git repository) to the .gogo configuration.',
    {
      path: z.string().describe('Relative path for the project directory'),
      url: z.string().describe('Git URL of the repository'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
    },
    async ({ path, url, cwd }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        const updated = addProject(config, path, url);
        await writeMetaConfig(metaDir, updated);
        return {
          content: [{ type: 'text' as const, text: `Project '${path}' added with URL '${url}'.` }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  server.tool(
    'gogo_project_remove',
    'Remove a project from the .gogo configuration (does not delete files).',
    {
      path: z.string().describe('Relative path of the project to remove'),
      cwd: z.string().optional().describe('Working directory (defaults to process.cwd())'),
    },
    async ({ path, cwd }) => {
      try {
        const { config, metaDir } = await resolveContext(cwd);
        if (!getProjectUrl(config, path)) {
          return {
            content: [{ type: 'text' as const, text: `Project '${path}' not found in configuration.` }],
            isError: true,
          };
        }
        const updated = removeProject(config, path);
        await writeMetaConfig(metaDir, updated);
        return {
          content: [{ type: 'text' as const, text: `Project '${path}' removed from configuration.` }],
        };
      } catch (error) {
        return errorResponse(error);
      }
    },
  );

  return server;
}

async function main() {
  const transport = new StdioServerTransport();
  const server = createServer();
  await server.connect(transport);
  console.error('gogo-meta MCP server running on stdio');
}

main().catch((error) => {
  console.error('Fatal error starting gogo-meta MCP server:', error);
  process.exit(1);
});
