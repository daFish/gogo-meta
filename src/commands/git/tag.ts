import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface TagOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  delete?: boolean;
  message?: string;
  annotate?: boolean;
  list?: boolean;
}

export async function tagCommand(name: string | undefined, options: TagOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let command: string;
  let actionMessage: string;

  if (name && options.delete) {
    command = `git tag -d "${name}"`;
    actionMessage = `Deleting tag "${name}" across repositories...`;
  } else if (name) {
    const parts = ['git', 'tag'];
    if (options.annotate || options.message) parts.push('-a');
    parts.push(`"${name}"`);
    if (options.message) {
      const escaped = options.message.replace(/"/g, '\\"');
      parts.push(`-m "${escaped}"`);
    }
    command = parts.join(' ');
    actionMessage = `Creating tag "${name}" across repositories...`;
  } else {
    command = options.list ? 'git tag -l' : 'git tag';
    actionMessage = 'Listing tags across repositories...';
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
