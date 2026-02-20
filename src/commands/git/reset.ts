import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface ResetOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  soft?: boolean;
  hard?: boolean;
  mixed?: boolean;
}

export async function resetCommand(target: string | undefined, options: ResetOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'reset'];
  if (options.soft) parts.push('--soft');
  else if (options.hard) parts.push('--hard');
  else if (options.mixed) parts.push('--mixed');
  if (target) parts.push(target);

  const command = parts.join(' ');

  output.info('Running git reset across repositories...');

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: false,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
