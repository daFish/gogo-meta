import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface CherryPickOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  abort?: boolean;
  continue?: boolean;
  noCommit?: boolean;
}

export async function cherryPickCommand(commits: string | undefined, options: CherryPickOptions = {}): Promise<void> {
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
    command = 'git cherry-pick --abort';
    actionMessage = 'Aborting cherry-pick across repositories...';
  } else if (options.continue) {
    command = 'git cherry-pick --continue';
    actionMessage = 'Continuing cherry-pick across repositories...';
  } else if (!commits) {
    throw new Error('Commit SHA(s) required for cherry-pick');
  } else {
    const parts = ['git', 'cherry-pick'];
    if (options.noCommit) parts.push('--no-commit');
    parts.push(commits);
    command = parts.join(' ');
    actionMessage = `Cherry-picking ${commits} across repositories...`;
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
