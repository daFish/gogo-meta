import pc from 'picocolors';

export const symbols = {
  success: pc.green('✓'),
  error: pc.red('✗'),
  warning: pc.yellow('⚠'),
  info: pc.blue('ℹ'),
  arrow: pc.cyan('→'),
  bullet: pc.gray('•'),
} as const;

export function success(message: string): void {
  console.log(`${symbols.success} ${message}`);
}

export function error(message: string): void {
  console.error(`${symbols.error} ${pc.red(message)}`);
}

export function warning(message: string): void {
  console.warn(`${symbols.warning} ${pc.yellow(message)}`);
}

export function info(message: string): void {
  console.log(`${symbols.info} ${message}`);
}

export function header(directory: string): void {
  console.log(`\n${symbols.arrow} ${pc.bold(pc.cyan(directory))}`);
}

export function dim(message: string): void {
  console.log(pc.dim(message));
}

export function bold(message: string): string {
  return pc.bold(message);
}

export function projectStatus(directory: string, status: 'success' | 'error', message?: string): void {
  const symbol = status === 'success' ? symbols.success : symbols.error;
  const colorFn = status === 'success' ? pc.green : pc.red;
  const suffix = message ? ` ${pc.dim(message)}` : '';
  console.log(`${symbol} ${colorFn(directory)}${suffix}`);
}

export function commandOutput(stdout: string, stderr: string): void {
  if (stdout.trim()) {
    console.log(stdout);
  }
  if (stderr.trim()) {
    console.error(pc.dim(stderr));
  }
}

export function summary(results: { success: number; failed: number; total: number }): void {
  const { success: successCount, failed, total } = results;

  console.log('');
  if (failed === 0) {
    console.log(`${symbols.success} ${pc.green(`All ${total} projects completed successfully`)}`);
  } else {
    console.log(
      `${symbols.warning} ${pc.yellow(`${successCount}/${total} projects succeeded, ${failed} failed`)}`
    );
  }
}

export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  const seconds = (ms / 1000).toFixed(1);
  return `${seconds}s`;
}
