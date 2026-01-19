import { Command } from 'commander';
import { createCommand } from './create.js';
import { importCommand } from './import.js';

export function registerProjectCommands(program: Command): void {
  const project = program
    .command('project')
    .description('Project management commands');

  project
    .command('create <folder> <url>')
    .description('Create and initialize a new child repository')
    .action(async (folder: string, url: string) => {
      await createCommand(folder, url);
    });

  project
    .command('import <folder> [url]')
    .description('Import an existing repository as a child project')
    .option('--no-clone', 'Register project without cloning')
    .action(async (folder: string, url: string | undefined, options: { clone?: boolean }) => {
      await importCommand(folder, url, { noClone: options.clone === false });
    });
}
