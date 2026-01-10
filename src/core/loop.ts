import { join } from 'node:path';
import { execute } from './executor.js';
import { applyFilters } from './filter.js';
import { getProjectPaths, readLoopRc } from './config.js';
import * as output from './output.js';
import type { MetaConfig, LoopOptions, LoopResult, ExecutorResult } from '../types/index.js';

const DEFAULT_CONCURRENCY = 4;

export interface LoopContext {
  config: MetaConfig;
  metaDir: string;
}

type CommandFn = (dir: string, projectPath: string) => Promise<ExecutorResult>;

async function runSequential(
  command: string | CommandFn,
  directories: string[],
  context: LoopContext,
  options: LoopOptions
): Promise<LoopResult[]> {
  const results: LoopResult[] = [];

  for (const projectPath of directories) {
    const absoluteDir = join(context.metaDir, projectPath);

    if (!options.suppressOutput) {
      output.header(projectPath);
    }

    const start = Date.now();

    let result: ExecutorResult;
    if (typeof command === 'function') {
      result = await command(absoluteDir, projectPath);
    } else {
      result = await execute(command, { cwd: absoluteDir });
    }

    const duration = Date.now() - start;

    if (!options.suppressOutput) {
      output.commandOutput(result.stdout, result.stderr);
    }

    results.push({
      directory: projectPath,
      result,
      success: result.exitCode === 0,
      duration,
    });
  }

  return results;
}

async function runParallel(
  command: string | CommandFn,
  directories: string[],
  context: LoopContext,
  options: LoopOptions
): Promise<LoopResult[]> {
  const concurrency = options.concurrency ?? DEFAULT_CONCURRENCY;
  const results: LoopResult[] = [];
  const pending: Promise<void>[] = [];
  let index = 0;

  const runOne = async (projectPath: string): Promise<void> => {
    const absoluteDir = join(context.metaDir, projectPath);
    const start = Date.now();

    let result: ExecutorResult;
    if (typeof command === 'function') {
      result = await command(absoluteDir, projectPath);
    } else {
      result = await execute(command, { cwd: absoluteDir });
    }

    const duration = Date.now() - start;

    results.push({
      directory: projectPath,
      result,
      success: result.exitCode === 0,
      duration,
    });
  };

  const runNext = async (): Promise<void> => {
    while (index < directories.length) {
      const currentIndex = index++;
      const projectPath = directories[currentIndex];
      if (projectPath) {
        await runOne(projectPath);
      }
    }
  };

  const workers = Math.min(concurrency, directories.length);
  for (let i = 0; i < workers; i++) {
    pending.push(runNext());
  }

  await Promise.all(pending);

  const orderedResults = directories
    .map((dir) => results.find((r) => r.directory === dir))
    .filter((r): r is LoopResult => r !== undefined);

  if (!options.suppressOutput) {
    for (const result of orderedResults) {
      output.header(result.directory);
      output.commandOutput(result.result.stdout, result.result.stderr);
    }
  }

  return orderedResults;
}

export async function loop(
  command: string | CommandFn,
  context: LoopContext,
  options: LoopOptions = {}
): Promise<LoopResult[]> {
  let directories = getProjectPaths(context.config);

  const loopRc = await readLoopRc(context.metaDir);
  if (loopRc?.ignore) {
    const ignoreSet = new Set(loopRc.ignore);
    directories = directories.filter((dir) => !ignoreSet.has(dir));
  }

  directories = applyFilters(directories, options);

  if (directories.length === 0) {
    output.warning('No projects match the specified filters');
    return [];
  }

  const results = options.parallel
    ? await runParallel(command, directories, context, options)
    : await runSequential(command, directories, context, options);

  if (!options.suppressOutput) {
    const successCount = results.filter((r) => r.success).length;
    const failedCount = results.length - successCount;
    output.summary({ success: successCount, failed: failedCount, total: results.length });
  }

  return results;
}

export function hasFailures(results: LoopResult[]): boolean {
  return results.some((r) => !r.success);
}

export function getExitCode(results: LoopResult[]): number {
  return hasFailures(results) ? 1 : 0;
}
