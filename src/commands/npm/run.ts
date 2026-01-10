import { join } from 'node:path';
import { readFile } from 'node:fs/promises';
import { readMetaConfig, getMetaDir, fileExists } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions, applyFilters } from '../../core/filter.js';
import * as output from '../../core/output.js';
import type { ExecutorResult } from '../../types/index.js';

interface RunOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
  ifPresent?: boolean;
}

async function hasScript(dir: string, scriptName: string): Promise<boolean> {
  const packageJsonPath = join(dir, 'package.json');
  if (!(await fileExists(packageJsonPath))) {
    return false;
  }

  try {
    const content = await readFile(packageJsonPath, 'utf-8');
    const packageJson = JSON.parse(content) as { scripts?: Record<string, string> };
    return scriptName in (packageJson.scripts ?? {});
  } catch {
    return false;
  }
}

export async function runCommand(script: string, options: RunOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  output.info(`Running "npm run ${script}" across repositories...`);

  const command = options.ifPresent
    ? async (dir: string, projectPath: string): Promise<ExecutorResult> => {
        const hasScriptResult = await hasScript(dir, script);
        if (!hasScriptResult) {
          return {
            exitCode: 0,
            stdout: `Script "${script}" not found, skipping`,
            stderr: '',
          };
        }

        const { execute } = await import('../../core/executor.js');
        return execute(`npm run ${script}`, { cwd: dir });
      }
    : `npm run ${script}`;

  let projectPaths = Object.keys(config.projects);
  projectPaths = applyFilters(projectPaths, filterOptions);

  if (options.ifPresent && typeof command === 'function') {
    const results = await loop(command, { config, metaDir }, {
      ...filterOptions,
      parallel: options.parallel,
      concurrency: options.concurrency,
    });

    const exitCode = getExitCode(results);
    if (exitCode !== 0) {
      process.exitCode = exitCode;
    }
  } else {
    const results = await loop(command as string, { config, metaDir }, {
      ...filterOptions,
      parallel: options.parallel,
      concurrency: options.concurrency,
    });

    const exitCode = getExitCode(results);
    if (exitCode !== 0) {
      process.exitCode = exitCode;
    }
  }
}
