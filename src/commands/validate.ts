import { Command } from 'commander';
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { parse as parseYaml } from 'yaml';
import { MetaConfigSchema, LoopRcSchema } from '../types/index.js';
import { detectFormat, LOOPRC_FILE } from '../core/config.js';
import * as output from '../core/output.js';

interface ValidationResult {
  file: string;
  valid: boolean;
  error?: string;
}

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

  if (entries.includes(LOOPRC_FILE)) {
    configFiles.push(LOOPRC_FILE);
  }

  return configFiles.sort();
}

export async function validateCommand(): Promise<void> {
  const cwd = process.cwd();
  const results: ValidationResult[] = [];

  const configFiles = await findConfigFiles(cwd);

  for (const filename of configFiles) {
    const filePath = join(cwd, filename);
    if (filename === LOOPRC_FILE) {
      results.push(await validateLoopRcFile(filePath));
    } else {
      results.push(await validateConfigFile(filePath, filename));
    }
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

  if (hasErrors) {
    throw new Error('Validation failed');
  }
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

async function validateLoopRcFile(filePath: string): Promise<ValidationResult> {
  try {
    const content = await readFile(filePath, 'utf-8');
    const parsed = JSON.parse(content);
    LoopRcSchema.parse(parsed);
    return { file: LOOPRC_FILE, valid: true };
  } catch (error) {
    if (error instanceof SyntaxError) {
      return { file: LOOPRC_FILE, valid: false, error: 'Invalid JSON' };
    }
    if (error instanceof Error && error.name === 'ZodError') {
      return { file: LOOPRC_FILE, valid: false, error: `Invalid structure: ${error.message}` };
    }
    return { file: LOOPRC_FILE, valid: false, error: String(error) };
  }
}

export function registerValidateCommand(program: Command): void {
  program
    .command('validate')
    .description('Validate all config files in the current directory')
    .action(async () => {
      await validateCommand();
    });
}
