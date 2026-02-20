import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { vol } from 'memfs';
import { diffCommand } from '../../src/commands/git/diff.js';
import { logCommand } from '../../src/commands/git/log.js';
import { fetchCommand } from '../../src/commands/git/fetch.js';
import { stashCommand } from '../../src/commands/git/stash.js';
import { tagCommand } from '../../src/commands/git/tag.js';
import { mergeCommand } from '../../src/commands/git/merge.js';
import { resetCommand } from '../../src/commands/git/reset.js';
import { pushCommand } from '../../src/commands/git/push.js';
import { commitCommand } from '../../src/commands/git/commit.js';
import { rebaseCommand } from '../../src/commands/git/rebase.js';
import { cherryPickCommand } from '../../src/commands/git/cherry-pick.js';
import { cleanCommand } from '../../src/commands/git/clean.js';

vi.mock('node:fs/promises', async () => {
  const memfs = await import('memfs');
  return memfs.fs.promises;
});

vi.mock('../../src/core/executor.js', () => ({
  execute: vi.fn(),
  executeSync: vi.fn(() => ({ exitCode: 0, stdout: '', stderr: '', timedOut: false })),
}));

vi.mock('../../src/core/output.js', () => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  header: vi.fn(),
  commandOutput: vi.fn(),
  summary: vi.fn(),
  projectStatus: vi.fn(),
  bold: vi.fn((s: string) => s),
}));

const CONFIG = {
  projects: {
    api: 'git@github.com:org/api.git',
    web: 'git@github.com:org/web.git',
  },
  ignore: [],
};

describe('extended git commands', () => {
  const mockExecute = vi.fn();

  beforeEach(async () => {
    vol.reset();
    vi.clearAllMocks();

    vi.spyOn(process, 'cwd').mockReturnValue('/project');

    const executor = await import('../../src/core/executor.js');
    (executor.execute as ReturnType<typeof vi.fn>).mockImplementation(mockExecute);

    vol.fromJSON({
      '/project/.gogo': JSON.stringify(CONFIG),
      '/project/api/.git': '',
      '/project/web/.git': '',
    });

    mockExecute.mockResolvedValue({ exitCode: 0, stdout: '', stderr: '' });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('diff command', () => {
    it('runs git diff across repos', async () => {
      await diffCommand(undefined);
      expect(mockExecute).toHaveBeenCalledWith('git diff', expect.objectContaining({ cwd: '/project/api' }));
      expect(mockExecute).toHaveBeenCalledWith('git diff', expect.objectContaining({ cwd: '/project/web' }));
    });

    it('supports --cached flag', async () => {
      await diffCommand(undefined, { cached: true });
      expect(mockExecute).toHaveBeenCalledWith('git diff --cached', expect.any(Object));
    });

    it('supports --stat flag', async () => {
      await diffCommand(undefined, { stat: true });
      expect(mockExecute).toHaveBeenCalledWith('git diff --stat', expect.any(Object));
    });

    it('supports --name-only flag', async () => {
      await diffCommand(undefined, { nameOnly: true });
      expect(mockExecute).toHaveBeenCalledWith('git diff --name-only', expect.any(Object));
    });

    it('supports diff target', async () => {
      await diffCommand('HEAD~1..HEAD');
      expect(mockExecute).toHaveBeenCalledWith('git diff HEAD~1..HEAD', expect.any(Object));
    });

    it('combines flags', async () => {
      await diffCommand('develop..HEAD', { stat: true, nameOnly: true });
      expect(mockExecute).toHaveBeenCalledWith('git diff --stat --name-only develop..HEAD', expect.any(Object));
    });
  });

  describe('log command', () => {
    it('runs git log across repos', async () => {
      await logCommand();
      expect(mockExecute).toHaveBeenCalledWith('git log', expect.objectContaining({ cwd: '/project/api' }));
    });

    it('supports --oneline', async () => {
      await logCommand({ oneline: true });
      expect(mockExecute).toHaveBeenCalledWith('git log --oneline', expect.any(Object));
    });

    it('supports -n limit', async () => {
      await logCommand({ number: 5 });
      expect(mockExecute).toHaveBeenCalledWith('git log -5', expect.any(Object));
    });

    it('supports --since', async () => {
      await logCommand({ since: '6 hours ago' });
      expect(mockExecute).toHaveBeenCalledWith('git log --since="6 hours ago"', expect.any(Object));
    });

    it('supports --format', async () => {
      await logCommand({ format: '%h %s' });
      expect(mockExecute).toHaveBeenCalledWith('git log --format="%h %s"', expect.any(Object));
    });

    it('combines options', async () => {
      await logCommand({ oneline: true, number: 10 });
      expect(mockExecute).toHaveBeenCalledWith('git log --oneline -10', expect.any(Object));
    });
  });

  describe('fetch command', () => {
    it('runs git fetch across repos', async () => {
      await fetchCommand();
      expect(mockExecute).toHaveBeenCalledWith('git fetch', expect.objectContaining({ cwd: '/project/api' }));
    });

    it('supports --all flag', async () => {
      await fetchCommand({ all: true });
      expect(mockExecute).toHaveBeenCalledWith('git fetch --all', expect.any(Object));
    });

    it('supports --prune flag', async () => {
      await fetchCommand({ prune: true });
      expect(mockExecute).toHaveBeenCalledWith('git fetch --prune', expect.any(Object));
    });

    it('supports --tags flag', async () => {
      await fetchCommand({ tags: true });
      expect(mockExecute).toHaveBeenCalledWith('git fetch --tags', expect.any(Object));
    });
  });

  describe('stash command', () => {
    it('runs git stash with no action', async () => {
      await stashCommand(undefined);
      expect(mockExecute).toHaveBeenCalledWith('git stash', expect.any(Object));
    });

    it('supports push with message', async () => {
      await stashCommand('push', { message: 'WIP: my changes' });
      expect(mockExecute).toHaveBeenCalledWith('git stash push -m "WIP: my changes"', expect.any(Object));
    });

    it('supports pop', async () => {
      await stashCommand('pop');
      expect(mockExecute).toHaveBeenCalledWith('git stash pop', expect.any(Object));
    });

    it('supports list', async () => {
      await stashCommand('list');
      expect(mockExecute).toHaveBeenCalledWith('git stash list', expect.any(Object));
    });

    it('supports drop', async () => {
      await stashCommand('drop');
      expect(mockExecute).toHaveBeenCalledWith('git stash drop', expect.any(Object));
    });

    it('supports show', async () => {
      await stashCommand('show');
      expect(mockExecute).toHaveBeenCalledWith('git stash show', expect.any(Object));
    });
  });

  describe('tag command', () => {
    it('lists tags', async () => {
      await tagCommand(undefined);
      expect(mockExecute).toHaveBeenCalledWith('git tag', expect.any(Object));
    });

    it('creates a lightweight tag', async () => {
      await tagCommand('v1.0.0');
      expect(mockExecute).toHaveBeenCalledWith('git tag "v1.0.0"', expect.any(Object));
    });

    it('creates an annotated tag with message', async () => {
      await tagCommand('v1.0.0', { message: 'Release 1.0.0' });
      expect(mockExecute).toHaveBeenCalledWith('git tag -a "v1.0.0" -m "Release 1.0.0"', expect.any(Object));
    });

    it('deletes a tag', async () => {
      await tagCommand('v1.0.0', { delete: true });
      expect(mockExecute).toHaveBeenCalledWith('git tag -d "v1.0.0"', expect.any(Object));
    });
  });

  describe('merge command', () => {
    it('merges a branch', async () => {
      await mergeCommand('develop');
      expect(mockExecute).toHaveBeenCalledWith('git merge develop', expect.any(Object));
    });

    it('supports --no-ff', async () => {
      await mergeCommand('develop', { noFf: true });
      expect(mockExecute).toHaveBeenCalledWith('git merge --no-ff develop', expect.any(Object));
    });

    it('supports --ff-only', async () => {
      await mergeCommand('develop', { ffOnly: true });
      expect(mockExecute).toHaveBeenCalledWith('git merge --ff-only develop', expect.any(Object));
    });

    it('supports --squash', async () => {
      await mergeCommand('feature', { squash: true });
      expect(mockExecute).toHaveBeenCalledWith('git merge --squash feature', expect.any(Object));
    });

    it('supports --abort', async () => {
      await mergeCommand(undefined, { abort: true });
      expect(mockExecute).toHaveBeenCalledWith('git merge --abort', expect.any(Object));
    });

    it('throws when branch is missing and not aborting', async () => {
      await expect(mergeCommand(undefined)).rejects.toThrow('Branch name is required');
    });
  });

  describe('reset command', () => {
    it('runs git reset', async () => {
      await resetCommand(undefined);
      expect(mockExecute).toHaveBeenCalledWith('git reset', expect.any(Object));
    });

    it('supports --soft', async () => {
      await resetCommand('HEAD~1', { soft: true });
      expect(mockExecute).toHaveBeenCalledWith('git reset --soft HEAD~1', expect.any(Object));
    });

    it('supports --hard', async () => {
      await resetCommand('HEAD~1', { hard: true });
      expect(mockExecute).toHaveBeenCalledWith('git reset --hard HEAD~1', expect.any(Object));
    });

    it('supports target without mode flag', async () => {
      await resetCommand('HEAD~3');
      expect(mockExecute).toHaveBeenCalledWith('git reset HEAD~3', expect.any(Object));
    });
  });

  describe('push command (enhanced)', () => {
    it('supports --force-with-lease', async () => {
      await pushCommand({ forceWithLease: true });
      expect(mockExecute).toHaveBeenCalledWith('git push --force-with-lease', expect.any(Object));
    });

    it('supports --force', async () => {
      await pushCommand({ force: true });
      expect(mockExecute).toHaveBeenCalledWith('git push --force', expect.any(Object));
    });

    it('supports --tags', async () => {
      await pushCommand({ tags: true });
      expect(mockExecute).toHaveBeenCalledWith('git push --tags', expect.any(Object));
    });

    it('supports --set-upstream', async () => {
      await pushCommand({ setUpstream: 'feature-branch' });
      expect(mockExecute).toHaveBeenCalledWith('git push -u origin feature-branch', expect.any(Object));
    });

    it('prefers --force-with-lease over --force', async () => {
      await pushCommand({ forceWithLease: true, force: true });
      expect(mockExecute).toHaveBeenCalledWith('git push --force-with-lease', expect.any(Object));
    });
  });

  describe('commit command (enhanced)', () => {
    it('supports --fixup', async () => {
      await commitCommand({ fixup: 'abc123' });
      expect(mockExecute).toHaveBeenCalledWith('git commit --fixup=abc123', expect.any(Object));
    });

    it('supports -a flag', async () => {
      await commitCommand({ all: true, message: 'update' });
      expect(mockExecute).toHaveBeenCalledWith('git commit -a -m "update"', expect.any(Object));
    });

    it('supports --amend', async () => {
      await commitCommand({ amend: true });
      expect(mockExecute).toHaveBeenCalledWith('git commit --amend --no-edit', expect.any(Object));
    });
  });

  describe('rebase command', () => {
    it('rebases onto a target', async () => {
      await rebaseCommand('main');
      expect(mockExecute).toHaveBeenCalledWith('git rebase main', expect.any(Object));
    });

    it('supports --autosquash (non-interactive)', async () => {
      await rebaseCommand('main', { autosquash: true });
      expect(mockExecute).toHaveBeenCalledWith(
        'GIT_SEQUENCE_EDITOR=true git rebase --autosquash main',
        expect.any(Object),
      );
    });

    it('supports --onto', async () => {
      await rebaseCommand('main', { onto: 'develop' });
      expect(mockExecute).toHaveBeenCalledWith('git rebase --onto develop main', expect.any(Object));
    });

    it('supports --abort', async () => {
      await rebaseCommand(undefined, { abort: true });
      expect(mockExecute).toHaveBeenCalledWith('git rebase --abort', expect.any(Object));
    });

    it('supports --continue', async () => {
      await rebaseCommand(undefined, { continue: true });
      expect(mockExecute).toHaveBeenCalledWith('git rebase --continue', expect.any(Object));
    });

    it('supports --skip', async () => {
      await rebaseCommand(undefined, { skip: true });
      expect(mockExecute).toHaveBeenCalledWith('git rebase --skip', expect.any(Object));
    });

    it('supports autosquash with onto', async () => {
      await rebaseCommand('HEAD~5', { autosquash: true, onto: 'main' });
      expect(mockExecute).toHaveBeenCalledWith(
        'GIT_SEQUENCE_EDITOR=true git rebase --autosquash --onto main HEAD~5',
        expect.any(Object),
      );
    });
  });

  describe('cherry-pick command', () => {
    it('cherry-picks a commit', async () => {
      await cherryPickCommand('abc123');
      expect(mockExecute).toHaveBeenCalledWith('git cherry-pick abc123', expect.any(Object));
    });

    it('supports --no-commit', async () => {
      await cherryPickCommand('abc123', { noCommit: true });
      expect(mockExecute).toHaveBeenCalledWith('git cherry-pick --no-commit abc123', expect.any(Object));
    });

    it('supports --abort', async () => {
      await cherryPickCommand(undefined, { abort: true });
      expect(mockExecute).toHaveBeenCalledWith('git cherry-pick --abort', expect.any(Object));
    });

    it('supports --continue', async () => {
      await cherryPickCommand(undefined, { continue: true });
      expect(mockExecute).toHaveBeenCalledWith('git cherry-pick --continue', expect.any(Object));
    });

    it('throws when no commits and not aborting/continuing', async () => {
      await expect(cherryPickCommand(undefined)).rejects.toThrow('Commit SHA(s) required');
    });
  });

  describe('clean command', () => {
    it('runs git clean with force', async () => {
      await cleanCommand({ force: true });
      expect(mockExecute).toHaveBeenCalledWith('git clean -f', expect.any(Object));
    });

    it('supports -d (directories)', async () => {
      await cleanCommand({ force: true, directories: true });
      expect(mockExecute).toHaveBeenCalledWith('git clean -f -d', expect.any(Object));
    });

    it('supports dry run', async () => {
      await cleanCommand({ dryRun: true });
      expect(mockExecute).toHaveBeenCalledWith('git clean -n', expect.any(Object));
    });

    it('supports -x (ignored files)', async () => {
      await cleanCommand({ force: true, ignored: true });
      expect(mockExecute).toHaveBeenCalledWith('git clean -f -x', expect.any(Object));
    });
  });
});
