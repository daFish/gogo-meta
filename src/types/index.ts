import { z } from 'zod';

export const MetaConfigSchema = z.object({
  projects: z.record(z.string(), z.string()),
  ignore: z.array(z.string()).default(['.git', 'node_modules', '.vagrant', '.vscode']),
});

export type MetaConfig = z.infer<typeof MetaConfigSchema>;

export const LoopRcSchema = z.object({
  ignore: z.array(z.string()).default([]),
});

export type LoopRc = z.infer<typeof LoopRcSchema>;

export interface FilterOptions {
  includeOnly?: string[] | undefined;
  excludeOnly?: string[] | undefined;
  includePattern?: RegExp | undefined;
  excludePattern?: RegExp | undefined;
}

export interface ExecutorOptions {
  cwd: string;
  env?: NodeJS.ProcessEnv | undefined;
  timeout?: number | undefined;
  shell?: boolean | undefined;
}

export interface ExecutorResult {
  exitCode: number;
  stdout: string;
  stderr: string;
  timedOut?: boolean | undefined;
}

export interface LoopOptions extends FilterOptions {
  parallel?: boolean | undefined;
  concurrency?: number | undefined;
  suppressOutput?: boolean | undefined;
}

export interface LoopResult {
  directory: string;
  result: ExecutorResult;
  success: boolean;
  duration: number;
}

export interface ProjectInfo {
  path: string;
  url: string;
  exists: boolean;
}

export type CommandHandler<T = void> = (options: T) => Promise<void>;
