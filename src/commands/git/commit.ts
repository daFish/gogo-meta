import { readMetaConfig, getMetaDir } from '../../core/config.js';
import { loop, getExitCode } from '../../core/loop.js';
import { createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface CommitOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  message?: string;
  fixup?: string;
  all?: boolean;
  amend?: boolean;
}

export async function commitCommand(options: CommitOptions): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const { config } = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  const parts = ['git', 'commit'];
  if (options.all) parts.push('-a');
  if (options.amend) {
    parts.push('--amend', '--no-edit');
  } else if (options.fixup) {
    parts.push(`--fixup=${options.fixup}`);
  } else if (options.message) {
    const escaped = options.message.replace(/"/g, '\\"');
    parts.push(`-m "${escaped}"`);
  }

  const command = parts.join(' ');

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
