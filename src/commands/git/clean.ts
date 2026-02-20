import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface CleanOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  force?: boolean;
  directories?: boolean;
  dryRun?: boolean;
  ignored?: boolean;
}

export async function cleanCommand(options: CleanOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'clean'];
  if (options.force) parts.push('-f');
  if (options.directories) parts.push('-d');
  if (options.dryRun) parts.push('-n');
  if (options.ignored) parts.push('-x');

  const command = parts.join(' ');

  output.info('Cleaning working directories across repositories...');

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
