import { Command } from 'commander';
import { installCommand } from './install.js';
import { linkCommand } from './link.js';
import { runCommand } from './run.js';

interface NpmCommandOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
}

export function registerNpmCommands(program: Command): void {
  const npm = program
    .command('npm')
    .description('NPM operations across all repositories');

  npm
    .command('install')
    .alias('i')
    .description('Run npm install in all projects')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .option('--concurrency <number>', 'Max parallel processes', parseInt)
    .action(async (options: NpmCommandOptions) => {
      await installCommand('install', options);
    });

  npm
    .command('ci')
    .description('Run npm ci in all projects')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .option('--concurrency <number>', 'Max parallel processes', parseInt)
    .action(async (options: NpmCommandOptions) => {
      await installCommand('ci', options);
    });

  npm
    .command('link')
    .description('Create npm links between projects')
    .option('--all', 'Link all projects bidirectionally')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (options: NpmCommandOptions & { all?: boolean }) => {
      await linkCommand(options);
    });

  npm
    .command('run <script>')
    .description('Run an npm script in all projects')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .option('--concurrency <number>', 'Max parallel processes', parseInt)
    .option('--if-present', 'Only run if script exists')
    .action(async (script: string, options: NpmCommandOptions & { ifPresent?: boolean }) => {
      await runCommand(script, options);
    });
}
