import { describe, it, expect } from 'vitest';
import {
  applyFilters,
  parseFilterList,
  parseFilterPattern,
  createFilterOptions,
  filterFromLoopRc,
} from '../../../src/core/filter.js';

describe('filter', () => {
  describe('applyFilters', () => {
    const directories = ['api', 'web', 'shared', 'docs', 'test-utils'];

    it('returns all directories when no filters applied', () => {
      const result = applyFilters(directories, {});
      expect(result).toEqual(directories);
    });

    describe('includeOnly', () => {
      it('filters to only specified directories', () => {
        const result = applyFilters(directories, {
          includeOnly: ['api', 'web'],
        });
        expect(result).toEqual(['api', 'web']);
      });

      it('handles non-existent directories gracefully', () => {
        const result = applyFilters(directories, {
          includeOnly: ['api', 'nonexistent'],
        });
        expect(result).toEqual(['api']);
      });

      it('matches on basename for nested paths', () => {
        const dirs = ['libs/api', 'libs/web'];
        const result = applyFilters(dirs, {
          includeOnly: ['api'],
        });
        expect(result).toEqual(['libs/api']);
      });
    });

    describe('excludeOnly', () => {
      it('removes specified directories', () => {
        const result = applyFilters(directories, {
          excludeOnly: ['docs', 'test-utils'],
        });
        expect(result).toEqual(['api', 'web', 'shared']);
      });

      it('handles non-existent directories gracefully', () => {
        const result = applyFilters(directories, {
          excludeOnly: ['nonexistent'],
        });
        expect(result).toEqual(directories);
      });
    });

    describe('includePattern', () => {
      it('includes directories matching regex', () => {
        const result = applyFilters(directories, {
          includePattern: /^(api|web)$/,
        });
        expect(result).toEqual(['api', 'web']);
      });

      it('works with partial matches', () => {
        const result = applyFilters(directories, {
          includePattern: /test/,
        });
        expect(result).toEqual(['test-utils']);
      });
    });

    describe('excludePattern', () => {
      it('excludes directories matching regex', () => {
        const result = applyFilters(directories, {
          excludePattern: /^test-/,
        });
        expect(result).toEqual(['api', 'web', 'shared', 'docs']);
      });

      it('can exclude multiple matches', () => {
        const result = applyFilters(directories, {
          excludePattern: /^(docs|shared)$/,
        });
        expect(result).toEqual(['api', 'web', 'test-utils']);
      });
    });

    describe('combined filters', () => {
      it('applies includeOnly before excludeOnly', () => {
        const result = applyFilters(directories, {
          includeOnly: ['api', 'web', 'shared'],
          excludeOnly: ['shared'],
        });
        expect(result).toEqual(['api', 'web']);
      });

      it('applies all filters in order', () => {
        const result = applyFilters(directories, {
          includeOnly: ['api', 'web', 'shared', 'test-utils'],
          excludeOnly: ['shared'],
          excludePattern: /test/,
        });
        expect(result).toEqual(['api', 'web']);
      });
    });
  });

  describe('parseFilterList', () => {
    it('parses comma-separated list', () => {
      const result = parseFilterList('api,web,shared');
      expect(result).toEqual(['api', 'web', 'shared']);
    });

    it('trims whitespace', () => {
      const result = parseFilterList(' api , web , shared ');
      expect(result).toEqual(['api', 'web', 'shared']);
    });

    it('filters empty strings', () => {
      const result = parseFilterList('api,,web');
      expect(result).toEqual(['api', 'web']);
    });

    it('returns undefined for empty input', () => {
      expect(parseFilterList('')).toBeUndefined();
      expect(parseFilterList(undefined)).toBeUndefined();
    });
  });

  describe('parseFilterPattern', () => {
    it('creates regex from string', () => {
      const result = parseFilterPattern('^test');
      expect(result).toBeInstanceOf(RegExp);
      expect(result?.test('test-utils')).toBe(true);
    });

    it('returns undefined for empty input', () => {
      expect(parseFilterPattern('')).toBeUndefined();
      expect(parseFilterPattern(undefined)).toBeUndefined();
    });

    it('throws for invalid regex', () => {
      expect(() => parseFilterPattern('[')).toThrow('Invalid regex pattern');
    });
  });

  describe('createFilterOptions', () => {
    it('creates filter options from string inputs', () => {
      const result = createFilterOptions({
        includeOnly: 'api,web',
        excludeOnly: 'docs',
        includePattern: '^lib',
        excludePattern: 'test',
      });

      expect(result.includeOnly).toEqual(['api', 'web']);
      expect(result.excludeOnly).toEqual(['docs']);
      expect(result.includePattern).toBeInstanceOf(RegExp);
      expect(result.excludePattern).toBeInstanceOf(RegExp);
    });

    it('handles undefined inputs', () => {
      const result = createFilterOptions({});

      expect(result.includeOnly).toBeUndefined();
      expect(result.excludeOnly).toBeUndefined();
      expect(result.includePattern).toBeUndefined();
      expect(result.excludePattern).toBeUndefined();
    });
  });

  describe('filterFromLoopRc', () => {
    it('removes directories in ignore list', () => {
      const dirs = ['api', 'web', 'node_modules', '.git'];
      const result = filterFromLoopRc(dirs, ['node_modules', '.git']);
      expect(result).toEqual(['api', 'web']);
    });

    it('returns all directories when ignore list is empty', () => {
      const dirs = ['api', 'web'];
      const result = filterFromLoopRc(dirs, []);
      expect(result).toEqual(dirs);
    });

    it('matches on basename', () => {
      const dirs = ['libs/api', 'libs/node_modules'];
      const result = filterFromLoopRc(dirs, ['node_modules']);
      expect(result).toEqual(['libs/api']);
    });
  });
});
