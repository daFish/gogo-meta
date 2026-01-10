import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface BranchOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  delete?: boolean;
  all?: boolean;
}

export async function branchCommand(name: string | undefined, options: BranchOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let command: string;
  let actionMessage: string;

  if (name) {
    if (options.delete) {
      command = `git branch -d "${name}"`;
      actionMessage = `Deleting branch "${name}" across repositories...`;
    } else {
      command = `git branch "${name}"`;
      actionMessage = `Creating branch "${name}" across repositories...`;
    }
  } else {
    command = options.all ? 'git branch -a' : 'git branch';
    actionMessage = 'Listing branches across repositories...';
  }

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
