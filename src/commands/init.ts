import { Command } from 'commander';
import { join } from 'node:path';
import { writeMetaConfig, createDefaultConfig, fileExists, META_FILE } from '../core/config.js';
import * as output from '../core/output.js';

interface InitOptions {
  force?: boolean;
}

export async function initCommand(options: InitOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaPath = join(cwd, META_FILE);

  if (await fileExists(metaPath)) {
    if (!options.force) {
      throw new Error(
        `${META_FILE} file already exists. Use --force to overwrite.`
      );
    }
    output.warning(`Overwriting existing ${META_FILE} file`);
  }

  const config = createDefaultConfig();
  await writeMetaConfig(cwd, config);

  output.success(`Created ${META_FILE} file in ${cwd}`);
  output.info('Add projects with: gogo project import <folder> <repo-url>');
}

export function registerInitCommand(program: Command): void {
  program
    .command('init')
    .description('Initialize a new gogo-meta repository')
    .option('-f, --force', 'Overwrite existing .meta file')
    .action(async (options: InitOptions) => {
      await initCommand(options);
    });
}
