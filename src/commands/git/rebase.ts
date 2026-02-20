import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface RebaseOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  autosquash?: boolean;
  abort?: boolean;
  continue?: boolean;
  skip?: boolean;
  onto?: string;
  interactive?: boolean;
}

export async function rebaseCommand(target: string | undefined, options: RebaseOptions = {}): Promise<void> {
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
    command = 'git rebase --abort';
    actionMessage = 'Aborting rebase across repositories...';
  } else if (options.continue) {
    command = 'git rebase --continue';
    actionMessage = 'Continuing rebase across repositories...';
  } else if (options.skip) {
    command = 'git rebase --skip';
    actionMessage = 'Skipping current patch across repositories...';
  } else {
    const parts: string[] = [];

    // For autosquash without user interaction, set GIT_SEQUENCE_EDITOR=true
    if (options.autosquash) {
      parts.push('GIT_SEQUENCE_EDITOR=true git rebase --autosquash');
    } else if (options.interactive) {
      parts.push('GIT_SEQUENCE_EDITOR=true git rebase');
    } else {
      parts.push('git rebase');
    }

    if (options.onto) parts.push(`--onto ${options.onto}`);
    if (target) parts.push(target);

    command = parts.join(' ');
    actionMessage = target
      ? `Rebasing onto "${target}" across repositories...`
      : 'Rebasing across repositories...';
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
