import { Command } from 'commander';
import { readMetaConfig, getMetaDir } from '../core/config.js';
import { loop, getExitCode } from '../core/loop.js';
import { createFilterOptions } from '../core/filter.js';
import * as output from '../core/output.js';

interface ExecOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
}

export async function execCommand(command: string, options: ExecOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  output.info(`Executing: ${output.bold(command)}`);

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
    concurrency: options.concurrency,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}

export function registerExecCommand(program: Command): void {
  program
    .command('exec <command>')
    .description('Execute a command in all project directories')
    .option('--include-only <dirs>', 'Only include specified directories (comma-separated)')
    .option('--exclude-only <dirs>', 'Exclude specified directories (comma-separated)')
    .option('--include-pattern <regex>', 'Include directories matching regex pattern')
    .option('--exclude-pattern <regex>', 'Exclude directories matching regex pattern')
    .option('--parallel', 'Execute commands in parallel')
    .option('--concurrency <number>', 'Max parallel processes (default: 4)', parseInt)
    .action(async (command: string, options: ExecOptions) => {
      await execCommand(command, options);
    });
}
