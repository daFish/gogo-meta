import { Command } from 'commander';
import { readMetaConfig, getMetaDir, getCommand, listCommands, type ResolvedCommand } from '../core/config.js';
import { loop, getExitCode } from '../core/loop.js';
import { createFilterOptions, parseFilterPattern } from '../core/filter.js';
import * as output from '../core/output.js';
import type { FilterOptions, LoopOptions } from '../types/index.js';

interface RunOptions {
  list?: boolean;
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  parallel?: boolean;
  concurrency?: number;
}

function formatCommandList(commands: Array<{ name: string; command: ResolvedCommand }>): void {
  if (commands.length === 0) {
    output.info('No commands defined in .gogo file');
    output.dim('  Add commands to your .gogo file:');
    output.dim('  "commands": { "build": "npm run build" }');
    return;
  }

  output.info('Available commands:');
  console.log('');

  const maxNameLen = Math.max(...commands.map((c) => c.name.length));

  for (const { name, command } of commands) {
    const paddedName = name.padEnd(maxNameLen);
    const desc = command.description ?? command.cmd;
    console.log(`  ${output.bold(paddedName)}  ${desc}`);
  }
}

export async function runCommand(name: string | undefined, options: RunOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);

  if (options.list || name === undefined) {
    formatCommandList(listCommands(config));
    return;
  }

  const commandDef = getCommand(config, name);

  if (!commandDef) {
    const available = Object.keys(config.commands ?? {});
    if (available.length === 0) {
      throw new Error(`Unknown command: "${name}". No commands are defined in .gogo file.`);
    }
    throw new Error(`Unknown command: "${name}". Available commands: ${available.join(', ')}`);
  }

  const cliFilterOptions = createFilterOptions(options);

  const mergedFilterOptions: FilterOptions = {
    includeOnly: cliFilterOptions.includeOnly ?? commandDef.includeOnly,
    excludeOnly: cliFilterOptions.excludeOnly ?? commandDef.excludeOnly,
    includePattern: cliFilterOptions.includePattern ?? parseFilterPattern(commandDef.includePattern),
    excludePattern: cliFilterOptions.excludePattern ?? parseFilterPattern(commandDef.excludePattern),
  };

  const loopOptions: LoopOptions = {
    ...mergedFilterOptions,
    parallel: options.parallel ?? commandDef.parallel,
    concurrency: options.concurrency ?? commandDef.concurrency,
  };

  output.info(`Running "${name}": ${output.bold(commandDef.cmd)}`);

  const results = await loop(commandDef.cmd, { config, metaDir }, loopOptions);

  const exitCode = getExitCode(results);
  if (exitCode !== 0) {
    process.exitCode = exitCode;
  }
}

export function registerRunCommand(program: Command): void {
  program
    .command('run [name]')
    .description('Run a predefined command from .gogo file')
    .option('-l, --list', 'List all available commands')
    .option('--include-only <dirs>', 'Only include specified directories (comma-separated)')
    .option('--exclude-only <dirs>', 'Exclude specified directories (comma-separated)')
    .option('--include-pattern <regex>', 'Include directories matching regex pattern')
    .option('--exclude-pattern <regex>', 'Exclude directories matching regex pattern')
    .option('--parallel', 'Execute commands in parallel')
    .option('--concurrency <number>', 'Max parallel processes (default: 4)', parseInt)
    .action(async (name: string | undefined, options: RunOptions) => {
      await runCommand(name, options);
    });
}
