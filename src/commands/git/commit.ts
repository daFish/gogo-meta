import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface CommitOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  message: string;
}

export async function commitCommand(options: CommitOptions): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const escapedMessage = options.message.replace(/"/g, '\\"');
  const command = `git commit -m "${escapedMessage}"`;

  output.info('Committing changes across repositories...');

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: false,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
