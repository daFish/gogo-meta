import { join, basename } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { execute } from '../../core/executor.js';
import {
  readMetaConfig,
  writeMetaConfig,
  getMetaDir,
  addProject,
  addToGitignore,
  fileExists,
} from '../../core/config.js';
import * as output from '../../core/output.js';

interface ImportOptions {
  noClone?: boolean;
}

async function getRemoteUrl(dir: string): Promise<string | null> {
  const result = await execute('git remote get-url origin', { cwd: dir });
  if (result.exitCode === 0 && result.stdout) {
    return result.stdout.trim();
  }
  return null;
}

export async function importCommand(
  folder: string,
  url?: string,
  options: ImportOptions = {}
): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const projectDir = join(metaDir, folder);
  const projectExists = await fileExists(projectDir);

  if (projectExists) {
    const existingUrl = await getRemoteUrl(projectDir);

    if (!existingUrl && !url) {
      throw new Error(
        `Directory "${folder}" exists but has no remote. Provide a URL to set one.`
      );
    }

    const finalUrl = url ?? existingUrl;

    if (url && existingUrl && url !== existingUrl) {
      output.warning(`Existing remote URL differs from provided URL`);
      output.info(`Existing: ${existingUrl}`);
      output.info(`Provided: ${url}`);
    }

    const config = await readMetaConfig(metaDir);
    const updatedConfig = addProject(config, folder, finalUrl!);
    await writeMetaConfig(metaDir, updatedConfig);

    output.success(`Imported existing project "${folder}"`);
    output.info(`Repository URL: ${finalUrl}`);
  } else {
    if (!url) {
      throw new Error('URL is required when importing a non-existent project');
    }

    if (options.noClone) {
      const config = await readMetaConfig(metaDir);
      const updatedConfig = addProject(config, folder, url);
      await writeMetaConfig(metaDir, updatedConfig);
      const added = await addToGitignore(metaDir, folder);
      output.success(`Registered project "${folder}" (not cloned)`);
      if (added) {
        output.info(`Added ${folder} to .gitignore`);
      }
      output.info(`Run "gogo git update" to clone missing projects`);
      return;
    }

    output.info(`Cloning ${url} into ${folder}...`);

    const parentDir = join(metaDir, folder, '..');
    await mkdir(parentDir, { recursive: true });

    const cloneResult = await execute(`git clone "${url}" "${basename(folder)}"`, {
      cwd: parentDir,
    });

    if (cloneResult.exitCode !== 0) {
      throw new Error(`Failed to clone repository: ${cloneResult.stderr}`);
    }

    const config = await readMetaConfig(metaDir);
    const updatedConfig = addProject(config, folder, url);
    await writeMetaConfig(metaDir, updatedConfig);

    output.success(`Imported project "${folder}"`);
  }

  const added = await addToGitignore(metaDir, folder);
  if (added) {
    output.info(`Added ${folder} to .gitignore`);
  }
}
