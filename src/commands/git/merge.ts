import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface MergeOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  noFf?: boolean;
  ffOnly?: boolean;
  abort?: boolean;
  squash?: boolean;
}

export async function mergeCommand(branch: string | undefined, options: MergeOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let command: string;
  let actionMessage: string;

  if (options.abort) {
    command = 'git merge --abort';
    actionMessage = 'Aborting merge across repositories...';
  } else if (!branch) {
    throw new Error('Branch name is required for merge');
  } else {
    const parts = ['git', 'merge'];
    if (options.noFf) parts.push('--no-ff');
    if (options.ffOnly) parts.push('--ff-only');
    if (options.squash) parts.push('--squash');
    parts.push(branch);
    command = parts.join(' ');
    actionMessage = `Merging "${branch}" across repositories...`;
  }

  output.info(actionMessage);

  const results = await loop(command, { config, metaDir }, {
    ...filterOptions,
    parallel: false,
  });

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}
