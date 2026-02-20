import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface StashOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  message?: string;
}

export async function stashCommand(action: string | undefined, options: StashOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let command: string;
  let actionMessage: string;

  switch (action) {
    case 'pop':
      command = 'git stash pop';
      actionMessage = 'Popping stash across repositories...';
      break;
    case 'list':
      command = 'git stash list';
      actionMessage = 'Listing stashes across repositories...';
      break;
    case 'drop':
      command = 'git stash drop';
      actionMessage = 'Dropping stash across repositories...';
      break;
    case 'show':
      command = 'git stash show';
      actionMessage = 'Showing stash across repositories...';
      break;
    case 'push':
    default: {
      const parts = ['git', 'stash', 'push'];
      if (options.message) {
        const escaped = options.message.replace(/"/g, '\\"');
        parts.push(`-m "${escaped}"`);
      }
      command = action === undefined && !options.message ? 'git stash' : parts.join(' ');
      actionMessage = 'Stashing changes across repositories...';
      break;
    }
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
