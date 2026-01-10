import { afterEach, vi } from 'vitest';

afterEach(() => {
  vi.restoreAllMocks();
  process.exitCode = undefined;
});
