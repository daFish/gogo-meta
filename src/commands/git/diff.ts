import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface DiffOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  cached?: boolean;
  stat?: boolean;
  nameOnly?: boolean;
}

export async function diffCommand(target: string | undefined, options: DiffOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'diff'];
  if (options.cached) parts.push('--cached');
  if (options.stat) parts.push('--stat');
  if (options.nameOnly) parts.push('--name-only');
  if (target) parts.push(target);

  const command = parts.join(' ');

  output.info('Running git diff across repositories...');

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
