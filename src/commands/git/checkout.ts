import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface CheckoutOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  create?: boolean;
}

export async function checkoutCommand(branch: string, options: CheckoutOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const flag = options.create ? '-b ' : '';
  const command = `git checkout ${flag}"${branch}"`;
  const actionMessage = options.create
    ? `Creating and checking out branch "${branch}" across repositories...`
    : `Checking out branch "${branch}" across repositories...`;

  output.info(actionMessage);

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: options.parallel,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
