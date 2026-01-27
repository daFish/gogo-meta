import { describe, it, expect } from 'vitest';
import { extractSshHost, extractUniqueSshHosts } from '../../../src/core/ssh.js';

describe('ssh', () => {
  describe('extractSshHost', () => {
    it('extracts host from git@host:path format', () => {
      expect(extractSshHost('git@github.com:user/repo.git')).toBe('github.com');
      expect(extractSshHost('git@gitlab.example.com:group/project.git')).toBe('gitlab.example.com');
    });

    it('extracts host from ssh:// format', () => {
      expect(extractSshHost('ssh://git@github.com/user/repo.git')).toBe('github.com');
      expect(extractSshHost('ssh://git@gitlab.example.com:2222/user/repo.git')).toBe(
        'gitlab.example.com'
      );
    });

    it('returns null for HTTPS URLs', () => {
      expect(extractSshHost('https://github.com/user/repo.git')).toBeNull();
      expect(extractSshHost('http://github.com/user/repo.git')).toBeNull();
    });

    it('returns null for file:// URLs', () => {
      expect(extractSshHost('file:///path/to/repo')).toBeNull();
    });

    it('handles various SSH user formats', () => {
      expect(extractSshHost('ssh@bitbucket.org:user/repo.git')).toBe('bitbucket.org');
    });
  });

  describe('extractUniqueSshHosts', () => {
    it('extracts unique hosts from multiple URLs', () => {
      const urls = [
        'git@github.com:user/repo1.git',
        'git@github.com:user/repo2.git',
        'git@gitlab.com:user/repo3.git',
      ];
      const hosts = extractUniqueSshHosts(urls);
      expect(hosts).toHaveLength(2);
      expect(hosts).toContain('github.com');
      expect(hosts).toContain('gitlab.com');
    });

    it('ignores non-SSH URLs', () => {
      const urls = [
        'git@github.com:user/repo.git',
        'https://github.com/user/repo2.git',
      ];
      const hosts = extractUniqueSshHosts(urls);
      expect(hosts).toEqual(['github.com']);
    });

    it('returns empty array for no SSH URLs', () => {
      const urls = [
        'https://github.com/user/repo1.git',
        'https://gitlab.com/user/repo2.git',
      ];
      const hosts = extractUniqueSshHosts(urls);
      expect(hosts).toEqual([]);
    });

    it('returns empty array for empty input', () => {
      expect(extractUniqueSshHosts([])).toEqual([]);
    });
  });
});
