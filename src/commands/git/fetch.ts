import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface FetchOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
  prune?: boolean;
  all?: boolean;
  tags?: boolean;
}

export async function fetchCommand(options: FetchOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'fetch'];
  if (options.all) parts.push('--all');
  if (options.prune) parts.push('--prune');
  if (options.tags) parts.push('--tags');

  const command = parts.join(' ');

  output.info('Fetching across repositories...');

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
