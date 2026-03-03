import { Command } from 'commander';
import { join } from 'node:path';
import { unlink } from 'node:fs/promises';
import { writeMetaConfig, createDefaultConfig, fileExists, META_FILE_CANDIDATES, filenameForFormat, type ConfigFormat } from '../core/config.js';
import * as output from '../core/output.js';

interface InitOptions {
  force?: boolean;
  format?: string;
}

export async function initCommand(options: InitOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const format: ConfigFormat = options.format === 'yaml' ? 'yaml' : 'json';

  const existingFiles: string[] = [];
  for (const candidate of META_FILE_CANDIDATES) {
    const candidatePath = join(cwd, candidate);
    if (await fileExists(candidatePath)) {
      existingFiles.push(candidate);
    }
  }

  if (existingFiles.length > 0) {
    if (!options.force) {
      throw new Error(
        `${existingFiles[0]} file already exists. Use --force to overwrite.`
      );
    }
    for (const candidate of existingFiles) {
      await unlink(join(cwd, candidate));
    }
    output.warning(`Overwriting existing config file`);
  }

  const config = createDefaultConfig();
  await writeMetaConfig(cwd, config, format);

  const filename = filenameForFormat(format);
  output.success(`Created ${filename} file in ${cwd}`);
  output.info('Add projects with: gogo project import <folder> <repo-url>');
}

export function registerInitCommand(program: Command): void {
  program
    .command('init')
    .description('Initialize a new gogo-meta repository')
    .option('-f, --force', 'Overwrite existing config file')
    .option('--format <format>', 'Config file format (json or yaml)', 'json')
    .action(async (options: InitOptions) => {
      if (options.format && !['json', 'yaml'].includes(options.format)) {
        throw new Error(`Invalid format "${options.format}". Use "json" or "yaml".`);
      }
      await initCommand(options);
    });
}
