import { readFile, writeFile, access } from 'node:fs/promises';
import { join, dirname } from 'node:path';
import { MetaConfigSchema, LoopRcSchema, type MetaConfig, type LoopRc } from '../types/index.js';

export const META_FILE = '.meta';
export const LOOPRC_FILE = '.looprc';

export class ConfigError extends Error {
  constructor(message: string, public readonly path?: string) {
    super(message);
    this.name = 'ConfigError';
  }
}

export async function fileExists(path: string): Promise<boolean> {
  try {
    await access(path);
    return true;
  } catch {
    return false;
  }
}

export async function findFileUp(filename: string, startDir: string): Promise<string | null> {
  let currentDir = startDir;

  while (true) {
    const filePath = join(currentDir, filename);
    if (await fileExists(filePath)) {
      return filePath;
    }

    const parentDir = dirname(currentDir);
    if (parentDir === currentDir) {
      return null;
    }
    currentDir = parentDir;
  }
}

export async function readMetaConfig(cwd: string): Promise<MetaConfig> {
  const metaPath = await findFileUp(META_FILE, cwd);

  if (!metaPath) {
    throw new ConfigError(
      `No ${META_FILE} file found. Run 'gogo init' to create one, or navigate to a directory with a ${META_FILE} file.`
    );
  }

  try {
    const content = await readFile(metaPath, 'utf-8');
    const parsed = JSON.parse(content);
    return MetaConfigSchema.parse(parsed);
  } catch (error) {
    if (error instanceof SyntaxError) {
      throw new ConfigError(`Invalid JSON in ${META_FILE} file`, metaPath);
    }
    if (error instanceof Error && error.name === 'ZodError') {
      throw new ConfigError(`Invalid ${META_FILE} file structure: ${error.message}`, metaPath);
    }
    throw error;
  }
}

export async function writeMetaConfig(cwd: string, config: MetaConfig): Promise<void> {
  const metaPath = join(cwd, META_FILE);
  const validated = MetaConfigSchema.parse(config);
  const content = JSON.stringify(validated, null, 2) + '\n';
  await writeFile(metaPath, content, 'utf-8');
}

export async function readLoopRc(cwd: string): Promise<LoopRc | null> {
  const looprcPath = await findFileUp(LOOPRC_FILE, cwd);

  if (!looprcPath) {
    return null;
  }

  try {
    const content = await readFile(looprcPath, 'utf-8');
    const parsed = JSON.parse(content);
    return LoopRcSchema.parse(parsed);
  } catch {
    return null;
  }
}

export function getMetaDir(cwd: string): Promise<string | null> {
  return findFileUp(META_FILE, cwd).then((path) => (path ? dirname(path) : null));
}

export function createDefaultConfig(): MetaConfig {
  return {
    projects: {},
    ignore: ['.git', 'node_modules', '.vagrant', '.vscode'],
  };
}

export function addProject(config: MetaConfig, path: string, url: string): MetaConfig {
  return {
    ...config,
    projects: {
      ...config.projects,
      [path]: url,
    },
  };
}

export function removeProject(config: MetaConfig, path: string): MetaConfig {
  const { [path]: _, ...remainingProjects } = config.projects;
  return {
    ...config,
    projects: remainingProjects,
  };
}

export function getProjectPaths(config: MetaConfig): string[] {
  return Object.keys(config.projects);
}

export function getProjectUrl(config: MetaConfig, path: string): string | undefined {
  return config.projects[path];
}
