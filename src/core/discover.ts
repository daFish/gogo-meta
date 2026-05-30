import { readdir } from 'node:fs/promises';
import { join, relative, sep } from 'node:path';

const GIT_DIR = '.git';

function toPosixRelative(rootDir: string, dir: string): string {
  return relative(rootDir, dir).split(sep).join('/');
}

/**
 * Walk `rootDir` and return the paths (relative to `rootDir`, POSIX-style) of
 * every directory that is a git repository. The root itself is never reported,
 * discovered repositories are not descended into, and any directory whose name
 * appears in `ignore` is skipped.
 */
export async function findGitRepos(rootDir: string, ignore: string[] = []): Promise<string[]> {
  const ignoreSet = new Set(ignore);
  const repos: string[] = [];

  async function walk(dir: string): Promise<void> {
    let entries;
    try {
      entries = await readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }

    const isRepo = entries.some((entry) => entry.name === GIT_DIR);
    if (isRepo && dir !== rootDir) {
      repos.push(toPosixRelative(rootDir, dir));
      return;
    }

    for (const entry of entries) {
      if (!entry.isDirectory()) {
        continue;
      }
      if (entry.name === GIT_DIR || ignoreSet.has(entry.name)) {
        continue;
      }
      await walk(join(dir, entry.name));
    }
  }

  await walk(rootDir);
  return repos.sort();
}
