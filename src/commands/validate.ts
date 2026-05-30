import { Command } from 'commander';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { parse as parseYaml } from 'yaml';
import { MetaConfigSchema, type MetaConfig } from '../types/index.js';
import { detectFormat, readMetaConfig, fileExists } from '../core/config.js';
import * as output from '../core/output.js';

interface ValidationResult {
  file: string;
  valid: boolean;
  error?: string;
}

const MISSING_DIRECTORY_HINT =
  "directory missing — run 'gogo migrate' if it moved, or 'gogo git update' to clone";

function isGogoConfigFile(filename: string): boolean {
  return filename === '.gogo' || filename.startsWith('.gogo.');
}

export async function findConfigFiles(cwd: string): Promise<string[]> {
  const entries = await readdir(cwd);
  const configFiles: string[] = [];

  for (const entry of entries) {
    if (isGogoConfigFile(entry)) {
      configFiles.push(entry);
    }
  }

  return configFiles.sort();
}

export async function validateCommand(): Promise<void> {
  const cwd = process.cwd();
  const results: ValidationResult[] = [];

  const configFiles = await findConfigFiles(cwd);

  for (const filename of configFiles) {
    const filePath = join(cwd, filename);
    results.push(await validateConfigFile(filePath, filename));
  }

  if (results.length === 0) {
    output.warning('No config files found in current directory');
    return;
  }

  const hasErrors = results.some((r) => !r.valid);

  for (const result of results) {
    if (result.valid) {
      output.projectStatus(result.file, 'success');
    } else {
      output.projectStatus(result.file, 'error', result.error);
    }
  }

  const workingCopyHasErrors = await validateWorkingCopy(cwd);

  if (hasErrors || workingCopyHasErrors) {
    throw new Error('Validation failed');
  }
}

async function validateWorkingCopy(cwd: string): Promise<boolean> {
  let config: MetaConfig;
  let metaDir: string;
  try {
    ({ config, metaDir } = await readMetaConfig(cwd));
  } catch {
    return false;
  }

  const projectPaths = Object.keys(config.projects);
  if (projectPaths.length === 0) {
    return false;
  }

  let hasErrors = false;
  for (const projectPath of projectPaths) {
    const projectDir = join(metaDir, projectPath);
    if (!(await fileExists(projectDir))) {
      output.projectStatus(projectPath, 'error', MISSING_DIRECTORY_HINT);
      hasErrors = true;
    }
  }

  if (!hasErrors) {
    output.success(`All ${projectPaths.length} project directories present`);
  }

  return hasErrors;
}

async function validateConfigFile(filePath: string, filename: string): Promise<ValidationResult> {
  const format = detectFormat(filePath);

  try {
    const content = await readFile(filePath, 'utf-8');
    const parsed = format === 'yaml' ? parseYaml(content) : JSON.parse(content);
    MetaConfigSchema.parse(parsed);
    return { file: filename, valid: true };
  } catch (error) {
    if (error instanceof SyntaxError) {
      return { file: filename, valid: false, error: 'Invalid JSON' };
    }
    if (error instanceof Error && error.name === 'YAMLParseError') {
      return { file: filename, valid: false, error: 'Invalid YAML' };
    }
    if (error instanceof Error && error.name === 'ZodError') {
      return { file: filename, valid: false, error: `Invalid structure: ${error.message}` };
    }
    return { file: filename, valid: false, error: String(error) };
  }
}

export function registerValidateCommand(program: Command): void {
  program
    .command('validate')
    .description('Validate config files and check that configured projects exist in the working copy')
    .action(async () => {
      await validateCommand();
    });
}
