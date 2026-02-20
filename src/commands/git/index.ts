import { Command } from 'commander';
import { cloneCommand } from './clone.js';
import { updateCommand } from './update.js';
import { statusCommand } from './status.js';
import { pullCommand } from './pull.js';
import { pushCommand } from './push.js';
import { branchCommand } from './branch.js';
import { checkoutCommand } from './checkout.js';
import { commitCommand } from './commit.js';
import { addCommand } from './add.js';
import { diffCommand } from './diff.js';
import { logCommand } from './log.js';
import { fetchCommand } from './fetch.js';
import { stashCommand } from './stash.js';
import { tagCommand } from './tag.js';
import { mergeCommand } from './merge.js';
import { resetCommand } from './reset.js';
import { rebaseCommand } from './rebase.js';
import { cherryPickCommand } from './cherry-pick.js';
import { cleanCommand } from './clean.js';

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
    .description('Clone any missing child repositories defined in .gogo')
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
    .command('diff [target]')
    .description('Show changes across all repositories')
    .option('--cached', 'Show staged changes')
    .option('--stat', 'Show diffstat summary')
    .option('--name-only', 'Show only file names')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (target: string | undefined, options: GitCommandOptions & { cached?: boolean; stat?: boolean; nameOnly?: boolean }) => {
      await diffCommand(target, options);
    });

  git
    .command('log')
    .description('Show commit log across all repositories')
    .option('-n, --number <count>', 'Limit number of commits', parseInt)
    .option('--oneline', 'Show compact one-line format')
    .option('--since <date>', 'Show commits since date')
    .option('--format <format>', 'Pretty-print format string')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (options: GitCommandOptions & { number?: number; oneline?: boolean; since?: string; format?: string }) => {
      await logCommand(options);
    });

  git
    .command('fetch')
    .description('Fetch from remotes across all repositories')
    .option('--all', 'Fetch from all remotes')
    .option('--prune', 'Remove remote-tracking refs that no longer exist')
    .option('--tags', 'Fetch all tags')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .option('--concurrency <number>', 'Max parallel operations', parseInt)
    .action(async (options: GitCommandOptions & { all?: boolean; prune?: boolean; tags?: boolean }) => {
      await fetchCommand(options);
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
    .option('--force-with-lease', 'Force push safely (reject if remote has new commits)')
    .option('-f, --force', 'Force push (use with caution)')
    .option('--tags', 'Push all tags')
    .option('-u, --set-upstream <branch>', 'Set upstream for the current branch')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (options: GitCommandOptions & { forceWithLease?: boolean; force?: boolean; tags?: boolean; setUpstream?: string }) => {
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
    .option('-m, --message <message>', 'Commit message')
    .option('--fixup <sha>', 'Create a fixup commit for the given SHA')
    .option('-a, --all', 'Stage all modified files before committing')
    .option('--amend', 'Amend the previous commit (no edit)')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (options: GitCommandOptions & { message?: string; fixup?: string; all?: boolean; amend?: boolean }) => {
      await commitCommand(options);
    });

  git
    .command('add [files]')
    .description('Stage files for commit across all repositories')
    .option('-A, --all', 'Stage all changes including untracked files')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (files: string | undefined, options: GitCommandOptions & { all?: boolean }) => {
      await addCommand(files, options);
    });

  git
    .command('merge [branch]')
    .description('Merge a branch in all repositories')
    .option('--no-ff', 'Create a merge commit even for fast-forward merges')
    .option('--ff-only', 'Only allow fast-forward merges')
    .option('--squash', 'Squash commits into a single commit')
    .option('--abort', 'Abort the current merge')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (branch: string | undefined, options: GitCommandOptions & { noFf?: boolean; ffOnly?: boolean; squash?: boolean; abort?: boolean }) => {
      await mergeCommand(branch, options);
    });

  git
    .command('tag [name]')
    .description('List, create, or delete tags')
    .option('-d, --delete', 'Delete the tag')
    .option('-a, --annotate', 'Create an annotated tag')
    .option('-m, --message <message>', 'Tag message (implies annotated)')
    .option('-l, --list', 'List tags')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (name: string | undefined, options: GitCommandOptions & { delete?: boolean; annotate?: boolean; message?: string; list?: boolean }) => {
      await tagCommand(name, options);
    });

  git
    .command('stash [action]')
    .description('Stash changes across all repositories (push/pop/list/drop/show)')
    .option('-m, --message <message>', 'Stash message')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (action: string | undefined, options: GitCommandOptions & { message?: string }) => {
      await stashCommand(action, options);
    });

  git
    .command('reset [target]')
    .description('Reset HEAD across all repositories')
    .option('--soft', 'Keep changes staged')
    .option('--hard', 'Discard all changes')
    .option('--mixed', 'Unstage changes (default)')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (target: string | undefined, options: GitCommandOptions & { soft?: boolean; hard?: boolean; mixed?: boolean }) => {
      await resetCommand(target, options);
    });

  git
    .command('rebase [target]')
    .description('Rebase across all repositories')
    .option('--autosquash', 'Automatically squash fixup commits (non-interactive)')
    .option('-i, --interactive', 'Run non-interactive rebase with GIT_SEQUENCE_EDITOR=true')
    .option('--onto <branch>', 'Rebase onto a different branch')
    .option('--abort', 'Abort the current rebase')
    .option('--continue', 'Continue after resolving conflicts')
    .option('--skip', 'Skip the current patch')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (target: string | undefined, options: GitCommandOptions & { autosquash?: boolean; interactive?: boolean; onto?: string; abort?: boolean; continue?: boolean; skip?: boolean }) => {
      await rebaseCommand(target, options);
    });

  git
    .command('cherry-pick [commits]')
    .description('Cherry-pick commits across all repositories')
    .option('--no-commit', 'Apply changes without creating commits')
    .option('--abort', 'Abort the current cherry-pick')
    .option('--continue', 'Continue after resolving conflicts')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .action(async (commits: string | undefined, options: GitCommandOptions & { noCommit?: boolean; abort?: boolean; continue?: boolean }) => {
      await cherryPickCommand(commits, options);
    });

  git
    .command('clean')
    .description('Clean untracked files across all repositories')
    .option('-f, --force', 'Force clean (required by git)')
    .option('-d, --directories', 'Also remove untracked directories')
    .option('-n, --dry-run', 'Show what would be removed')
    .option('-x, --ignored', 'Also remove ignored files')
    .option('--include-only <dirs>', 'Only include specified directories')
    .option('--exclude-only <dirs>', 'Exclude specified directories')
    .option('--parallel', 'Run in parallel')
    .action(async (options: GitCommandOptions & { force?: boolean; directories?: boolean; dryRun?: boolean; ignored?: boolean }) => {
      await cleanCommand(options);
    });
}
