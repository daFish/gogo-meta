import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface PushOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
}

export async function pushCommand(options: PushOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  output.info('Pushing changes across repositories...');

  const results = await loop('git push', { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
