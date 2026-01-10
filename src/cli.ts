import { Command } from 'commander';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { registerInitCommand } from './commands/init.js';
import { registerExecCommand } from './commands/exec.js';
import { registerRunCommand } from './commands/run.js';
import { registerGitCommands } from './commands/git/index.js';
import { registerProjectCommands } from './commands/project/index.js';
import { registerNpmCommands } from './commands/npm/index.js';
import * as output from './core/output.js';

function getVersion(): string {
  try {
    const __dirname = dirname(fileURLToPath(import.meta.url));
    const packageJsonPath = join(__dirname, '..', 'package.json');
    const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf-8'));
    return packageJson.version;
  } catch {
    return '0.0.0';
  }
}

export function createProgram(): Command {
  const program = new Command();

  program
    .name('gogo')
    .description('A modern CLI tool for managing multi-repository projects')
    .version(getVersion())
    .option('--include-only <dirs>', 'Only include specified directories (comma-separated)')
    .option('--exclude-only <dirs>', 'Exclude specified directories (comma-separated)')
    .option('--include-pattern <regex>', 'Include directories matching regex pattern')
    .option('--exclude-pattern <regex>', 'Exclude directories matching regex pattern')
    .option('--parallel', 'Execute commands in parallel')
    .option('--concurrency <number>', 'Max parallel processes (default: 4)', parseInt);

  registerInitCommand(program);
  registerExecCommand(program);
  registerRunCommand(program);
  registerGitCommands(program);
  registerProjectCommands(program);
  registerNpmCommands(program);

  return program;
}

async function main(): Promise<void> {
  const program = createProgram();

  try {
    await program.parseAsync(process.argv);
  } catch (error) {
    if (error instanceof Error) {
      output.error(error.message);
    } else {
      output.error('An unexpected error occurred');
    }
    process.exit(1);
  }
}

main();
