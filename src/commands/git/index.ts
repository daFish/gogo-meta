import { Command } from 'commander';
import { cloneCommand } from './clone.js';
import { updateCommand } from './update.js';
import { statusCommand } from './status.js';
import { pullCommand } from './pull.js';
import { pushCommand } from './push.js';
import { branchCommand } from './branch.js';
import { checkoutCommand } from './checkout.js';
import { commitCommand } from './commit.js';

interface GitCommandOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
}

export function registerGitCommands(program: Command): void {
  const git = program
    .command('git')
    .description('Git operations across all repositories');

  git
    .command('clone <url>')
    .description('Clone a meta repository and all child repositories')
    .option('-d, --directory <dir>', 'Target directory name')
    .action(async (url: string, options: { directory?: string }) => {
      await cloneCommand(url, options);
    });

  git
    .command('update')
    .description('Clone any missing child repositories defined in .meta')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Clone in parallel')
    .option('--concurrency <number>', 'Max parallel clones', parseInt)
    .action(async (options: GitCommandOptions) => {
      await updateCommand(options);
    });

  git
    .command('status')
    .description('Show git status across all repositories')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (options: GitCommandOptions) => {
      await statusCommand(options);
    });

  git
    .command('pull')
    .description('Pull changes in all repositories')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .option('--concurrency <number>', 'Max parallel operations', parseInt)
    .action(async (options: GitCommandOptions) => {
      await pullCommand(options);
    });

  git
    .command('push')
    .description('Push changes in all repositories')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (options: GitCommandOptions) => {
      await pushCommand(options);
    });

  git
    .command('branch [name]')
    .description('List, create, or delete branches')
    .option('-d, --delete', 'Delete the branch')
    .option('-a, --all', 'List all branches (local and remote)')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (name: string | undefined, options: GitCommandOptions & { delete?: boolean; all?: boolean }) => {
      await branchCommand(name, options);
    });

  git
    .command('checkout <branch>')
    .description('Checkout a branch in all repositories')
    .option('-b, --create', 'Create the branch if it does not exist')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (branch: string, options: GitCommandOptions & { create?: boolean }) => {
      await checkoutCommand(branch, options);
    });

  git
    .command('commit')
    .description('Commit changes in all repositories')
    .requiredOption('-m, --message <message>', 'Commit message')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (options: GitCommandOptions & { message: string }) => {
      await commitCommand(options);
    });
}
