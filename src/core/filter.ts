import { basename } from 'node:path';
import type { FilterOptions } from '../types/index.js';

export function applyFilters(directories: string[], options: FilterOptions): string[] {
  let result = [...directories];

  if (options.includeOnly && options.includeOnly.length > 0) {
    const includeSet = new Set(options.includeOnly);
    result = result.filter((dir) => includeSet.has(dir) || includeSet.has(basename(dir)));
  }

  if (options.excludeOnly && options.excludeOnly.length > 0) {
    const excludeSet = new Set(options.excludeOnly);
    result = result.filter((dir) => !excludeSet.has(dir) && !excludeSet.has(basename(dir)));
  }

  if (options.includePattern) {
    result = result.filter((dir) => options.includePattern!.test(dir));
  }

  if (options.excludePattern) {
    result = result.filter((dir) => !options.excludePattern!.test(dir));
  }

  return result;
}

export function parseFilterList(input: string | undefined): string[] | undefined {
  if (!input) {
    return undefined;
  }
  return input
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export function parseFilterPattern(input: string | undefined): RegExp | undefined {
  if (!input) {
    return undefined;
  }
  try {
    return new RegExp(input);
  } catch {
    throw new Error(`Invalid regex pattern: ${input}`);
  }
}

export function createFilterOptions(options: {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
}): FilterOptions {
  return {
    includeOnly: parseFilterList(options.includeOnly),
    excludeOnly: parseFilterList(options.excludeOnly),
    includePattern: parseFilterPattern(options.includePattern),
    excludePattern: parseFilterPattern(options.excludePattern),
  };
}

export function filterFromLoopRc(
  directories: string[],
  ignore: string[]
): string[] {
  if (ignore.length === 0) {
    return directories;
  }

  const ignoreSet = new Set(ignore);
  return directories.filter((dir) => {
    const dirBasename = basename(dir);
    return !ignoreSet.has(dir) && !ignoreSet.has(dirBasename);
  });
}
