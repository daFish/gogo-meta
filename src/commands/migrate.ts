import { Command } from 'commander';
import { mkdir, rename, readdir, rmdir } from 'node:fs/promises';
import { join, dirname, resolve, sep } from 'node:path';
import { execute } from '../core/executor.js';
import {
  readMetaConfig,
  getMetaDir,
  fileExists,
  addToGitignore,
  removeFromGitignore,
} from '../core/config.js';
import { findGitRepos } from '../core/discover.js';
import * as output from '../core/output.js';

interface MigrateOptions {
  dryRun?: boolean;
}

interface Move {
  from: string;
  to: string;
}

interface Conflict {
  path: string;
  found: string | null;
}

async function pruneEmptyParents(metaDir: string, movedFrom: string): Promise<void> {
  const root = resolve(metaDir);
  let dir = resolve(dirname(join(metaDir, movedFrom)));

  while (dir !== root && dir.startsWith(root + sep)) {
    let entries: string[];
    try {
      entries = await readdir(dir);
    } catch {
      return;
    }
    if (entries.length > 0) {
      return;
    }
    await rmdir(dir);
    dir = dirname(dir);
  }
}

async function getRemoteUrl(dir: string): Promise<string | null> {
  const result = await execute('git remote get-url origin', { cwd: dir });
  if (result.exitCode === 0 && result.stdout) {
    return result.stdout.trim();
  }
  return null;
}

async function mapUrlsToCurrentPaths(
  metaDir: string,
  ignore: string[]
): Promise<{ urlToPath: Map<string, string>; ambiguousUrls: Set<string> }> {
  const repoPaths = await findGitRepos(metaDir, ignore);
  const urlToPath = new Map<string, string>();
  const ambiguousUrls = new Set<string>();

  for (const repoPath of repoPaths) {
    const url = await getRemoteUrl(join(metaDir, repoPath));
    if (!url) {
      continue;
    }
    if (urlToPath.has(url)) {
      ambiguousUrls.add(url);
      continue;
    }
    urlToPath.set(url, repoPath);
  }

  return { urlToPath, ambiguousUrls };
}

export async function migrateCommand(options: MigrateOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const { config } = await readMetaConfig(cwd);
  const desired = Object.entries(config.projects);

  if (desired.length === 0) {
    output.success('Working copy already matches configuration');
    return;
  }

  const { urlToPath, ambiguousUrls } = await mapUrlsToCurrentPaths(metaDir, config.ignore);

  const moves: Move[] = [];
  const missing: string[] = [];
  const conflicts: Conflict[] = [];
  const ambiguous: string[] = [];

  for (const [projectPath, url] of desired) {
    const targetDir = join(metaDir, projectPath);

    if (await fileExists(targetDir)) {
      const targetRemote = await getRemoteUrl(targetDir);
      if (targetRemote !== url) {
        conflicts.push({ path: projectPath, found: targetRemote });
      }
      continue;
    }

    const currentPath = urlToPath.get(url);
    if (ambiguousUrls.has(url)) {
      ambiguous.push(projectPath);
    } else if (currentPath && currentPath !== projectPath) {
      moves.push({ from: currentPath, to: projectPath });
    } else {
      missing.push(projectPath);
    }
  }

  if (conflicts.length > 0) {
    for (const conflict of conflicts) {
      output.projectStatus(
        conflict.path,
        'error',
        `occupied by a different repository (found ${conflict.found ?? 'no remote'})`
      );
    }
    throw new Error(
      'Migration aborted: one or more target paths are occupied by a different repository'
    );
  }

  if (moves.length === 0 && missing.length === 0 && ambiguous.length === 0) {
    output.success('Working copy already matches configuration');
    return;
  }

  for (const move of moves) {
    if (options.dryRun) {
      output.info(`Would move ${move.from} → ${move.to}`);
      continue;
    }
    const targetDir = join(metaDir, move.to);
    await mkdir(dirname(targetDir), { recursive: true });
    await rename(join(metaDir, move.from), targetDir);
    await pruneEmptyParents(metaDir, move.from);
    await removeFromGitignore(metaDir, move.from);
    await addToGitignore(metaDir, move.to);
    output.projectStatus(move.to, 'success', `moved from ${move.from}`);
  }

  for (const projectPath of ambiguous) {
    output.warning(
      `${projectPath}: multiple working-copy directories share its repository URL — resolve manually`
    );
  }

  for (const projectPath of missing) {
    output.warning(`${projectPath} not found in working copy — run 'gogo git update' to clone`);
  }

  if (options.dryRun) {
    output.info(`Dry run: ${moves.length} move(s) pending`);
    return;
  }

  if (missing.length > 0 || ambiguous.length > 0) {
    process.exitCode = 1;
  }
}

export function registerMigrateCommand(program: Command): void {
  program
    .command('migrate')
    .description('Move/rename working-copy directories to match the configuration')
    .option('--dry-run', 'Show what would be moved without changing anything')
    .action(async (options: { dryRun?: boolean }) => {
      await migrateCommand({ dryRun: options.dryRun ?? false });
    });
}
