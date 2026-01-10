import { join, basename } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { execute } from '../../core/executor.js';
import { readMetaConfig, fileExists } from '../../core/config.js';
import * as output from '../../core/output.js';

interface CloneOptions {
  directory?: string;
}

function extractRepoName(url: string): string {
  const match = url.match(/\/([^/]+?)(\.git)?$/);
  return match?.[1] ?? 'repo';
}

export async function cloneCommand(url: string, options: CloneOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const repoName = options.directory ?? extractRepoName(url);
  const targetDir = join(cwd, repoName);

  if (await fileExists(targetDir)) {
    throw new Error(`Directory "${repoName}" already exists`);
  }

  output.info(`Cloning meta repository: ${url}`);

  const cloneResult = await execute(`git clone "${url}" "${repoName}"`, { cwd });

  if (cloneResult.exitCode !== 0) {
    output.error(`Failed to clone meta repository`);
    output.commandOutput(cloneResult.stdout, cloneResult.stderr);
    process.exitCode = 1;
    return;
  }

  output.success(`Cloned meta repository to ${repoName}`);

  const metaPath = join(targetDir, '.gogo');
  if (!(await fileExists(metaPath))) {
    output.warning('No .gogo file found in cloned repository');
    return;
  }

  const config = await readMetaConfig(targetDir);
  const projects = Object.entries(config.projects);

  if (projects.length === 0) {
    output.info('No child repositories defined in .gogo');
    return;
  }

  output.info(`Cloning ${projects.length} child repositories...`);

  let successCount = 0;
  let failCount = 0;

  for (const [projectPath, projectUrl] of projects) {
    const projectDir = join(targetDir, projectPath);

    if (await fileExists(projectDir)) {
      output.projectStatus(projectPath, 'success', 'already exists');
      successCount++;
      continue;
    }

    const parentDir = join(targetDir, projectPath, '..');
    await mkdir(parentDir, { recursive: true });

    const result = await execute(`git clone "${projectUrl}" "${basename(projectPath)}"`, {
      cwd: parentDir,
    });

    if (result.exitCode === 0) {
      output.projectStatus(projectPath, 'success', 'cloned');
      successCount++;
    } else {
      output.projectStatus(projectPath, 'error', result.stderr || 'clone failed');
      failCount++;
    }
  }

  output.summary({ success: successCount, failed: failCount, total: projects.length });

  if (failCount > 0) {
    process.exitCode = 1;
  }
}
