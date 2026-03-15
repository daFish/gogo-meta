import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface LogOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  number?: number;
  oneline?: boolean;
  since?: string;
  format?: string;
}

export async function logCommand(options: LogOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'log'];
  if (options.oneline) parts.push('--oneline');
  if (options.number) parts.push(`-${options.number}`);
  if (options.since) parts.push(`--since="${options.since}"`);
  if (options.format) parts.push(`--format="${options.format}"`);

  const command = parts.join(' ');

  output.info('Running git log across repositories...');

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
