import { join } from 'node:path';
import { mkdir, appendFile } from 'node:fs/promises';
import { execute } from '../../core/executor.js';
import {
  readMetaConfig,
  writeMetaConfig,
  getMetaDir,
  addProject,
  fileExists,
} from '../../core/config.js';
import * as output from '../../core/output.js';

export async function createCommand(folder: string, url: string): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const projectDir = join(metaDir, folder);

  if (await fileExists(projectDir)) {
    throw new Error(`Directory "${folder}" already exists`);
  }

  output.info(`Creating new project: ${folder}`);

  await mkdir(projectDir, { recursive: true });

  const initResult = await execute('git init', { cwd: projectDir });
  if (initResult.exitCode !== 0) {
    throw new Error(`Failed to initialize git repository: ${initResult.stderr}`);
  }

  const remoteResult = await execute(`git remote add origin "${url}"`, { cwd: projectDir });
  if (remoteResult.exitCode !== 0) {
    throw new Error(`Failed to add remote: ${remoteResult.stderr}`);
  }

  const config = await readMetaConfig(metaDir);
  const updatedConfig = addProject(config, folder, url);
  await writeMetaConfig(metaDir, updatedConfig);

  const gitignorePath = join(metaDir, '.gitignore');
  if (await fileExists(gitignorePath)) {
    await appendFile(gitignorePath, `\n${folder}\n`);
    output.info(`Added ${folder} to .gitignore`);
  }

  output.success(`Created project "${folder}"`);
  output.info(`Repository initialized with remote: ${url}`);
}
