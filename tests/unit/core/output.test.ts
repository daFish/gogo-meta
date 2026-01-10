import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  symbols,
  success,
  error,
  warning,
  info,
  header,
  projectStatus,
  summary,
  formatDuration,
} from '../../../src/core/output.js';

describe('output', () => {
  beforeEach(() => {
    vi.spyOn(console, 'log').mockImplementation(() => {});
    vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.spyOn(console, 'warn').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('symbols', () => {
    it('has success symbol', () => {
      expect(symbols.success).toBeDefined();
      expect(symbols.success).toContain('✓');
    });

    it('has error symbol', () => {
      expect(symbols.error).toBeDefined();
      expect(symbols.error).toContain('✗');
    });

    it('has warning symbol', () => {
      expect(symbols.warning).toBeDefined();
      expect(symbols.warning).toContain('⚠');
    });

    it('has info symbol', () => {
      expect(symbols.info).toBeDefined();
      expect(symbols.info).toContain('ℹ');
    });
  });

  describe('success', () => {
    it('logs message with success symbol', () => {
      success('Test message');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('Test message'));
    });
  });

  describe('error', () => {
    it('logs message to stderr', () => {
      error('Error message');
      expect(console.error).toHaveBeenCalledWith(expect.stringContaining('Error message'));
    });
  });

  describe('warning', () => {
    it('logs message with warning', () => {
      warning('Warning message');
      expect(console.warn).toHaveBeenCalledWith(expect.stringContaining('Warning message'));
    });
  });

  describe('info', () => {
    it('logs message with info symbol', () => {
      info('Info message');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('Info message'));
    });
  });

  describe('header', () => {
    it('logs directory name', () => {
      header('libs/core');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('libs/core'));
    });
  });

  describe('projectStatus', () => {
    it('logs success status', () => {
      projectStatus('api', 'success');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('api'));
    });

    it('logs error status', () => {
      projectStatus('api', 'error', 'failed');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('api'));
    });

    it('includes optional message', () => {
      projectStatus('api', 'success', 'cloned');
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('cloned'));
    });
  });

  describe('summary', () => {
    it('shows all success message', () => {
      summary({ success: 3, failed: 0, total: 3 });
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('3'));
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('successfully'));
    });

    it('shows partial success message', () => {
      summary({ success: 2, failed: 1, total: 3 });
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('2/3'));
      expect(console.log).toHaveBeenCalledWith(expect.stringContaining('1 failed'));
    });
  });

  describe('formatDuration', () => {
    it('formats milliseconds', () => {
      expect(formatDuration(500)).toBe('500ms');
    });

    it('formats seconds', () => {
      expect(formatDuration(1500)).toBe('1.5s');
    });

    it('formats exactly 1 second', () => {
      expect(formatDuration(1000)).toBe('1.0s');
    });

    it('handles zero', () => {
      expect(formatDuration(0)).toBe('0ms');
    });
  });
});
