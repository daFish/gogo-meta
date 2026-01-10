import { join, basename } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { execute } from '../../core/executor.js';
import { readMetaConfig, getMetaDir, fileExists } from '../../core/config.js';
import { applyFilters, createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface UpdateOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
}

export async function updateCommand(options: UpdateOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let projectEntries = Object.entries(config.projects);
  const projectPaths = projectEntries.map(([path]) => path);
  const filteredPaths = applyFilters(projectPaths, filterOptions);
  projectEntries = projectEntries.filter(([path]) => filteredPaths.includes(path));

  if (projectEntries.length === 0) {
    output.warning('No projects match the specified filters');
    return;
  }

  output.info(`Checking ${projectEntries.length} repositories...`);

  const missing: Array<[string, string]> = [];

  for (const [projectPath, projectUrl] of projectEntries) {
    const projectDir = join(metaDir, projectPath);
    if (!(await fileExists(projectDir))) {
      missing.push([projectPath, projectUrl]);
    }
  }

  if (missing.length === 0) {
    output.success('All repositories are already cloned');
    return;
  }

  output.info(`Cloning ${missing.length} missing repositories...`);

  let successCount = 0;
  let failCount = 0;

  const cloneOne = async ([projectPath, projectUrl]: [string, string]): Promise<boolean> => {
    const projectDir = join(metaDir, projectPath);
    const parentDir = join(projectDir, '..');

    await mkdir(parentDir, { recursive: true });

    const result = await execute(`git clone "${projectUrl}" "${basename(projectPath)}"`, {
      cwd: parentDir,
    });

    if (result.exitCode === 0) {
      output.projectStatus(projectPath, 'success', 'cloned');
      return true;
    } else {
      output.projectStatus(projectPath, 'error', result.stderr || 'clone failed');
      return false;
    }
  };

  if (options.parallel) {
    const concurrency = options.concurrency ?? 4;
    const results: boolean[] = [];

    for (let i = 0; i < missing.length; i += concurrency) {
      const batch = missing.slice(i, i + concurrency);
      const batchResults = await Promise.all(batch.map(cloneOne));
      results.push(...batchResults);
    }

    successCount = results.filter(Boolean).length;
    failCount = results.filter((r) => !r).length;
  } else {
    for (const entry of missing) {
      const success = await cloneOne(entry);
      if (success) {
        successCount++;
      } else {
        failCount++;
      }
    }
  }

  output.summary({ success: successCount, failed: failCount, total: missing.length });

  if (failCount > 0) {
    process.exitCode = 1;
  }
}
