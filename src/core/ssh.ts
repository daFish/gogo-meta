import { homedir } from 'node:os';
import { join } from 'node:path';
import { readFile } from 'node:fs/promises';
import { executeSync } from './executor.js';
import * as output from './output.js';

/**
 * Extracts the SSH host from a git URL.
 * Supports formats like:
 *   - git@github.com:user/repo.git
 *   - ssh://git@github.com/user/repo.git
 *   - git@gitlab.example.com:2222:user/repo.git (custom port)
 *
 * Returns null for non-SSH URLs (https://, file://, etc.)
 */
export function extractSshHost(url: string): string | null {
  // Skip non-SSH URLs
  if (url.startsWith('https://') || url.startsWith('http://') || url.startsWith('file://')) {
    return null;
  }

  // Handle ssh:// format: ssh://git@host/path or ssh://git@host:port/path
  const sshMatch = url.match(/^ssh:\/\/[^@]+@([^/:]+)/);
  if (sshMatch) {
    return sshMatch[1] ?? null;
  }

  // Handle git@host:path format
  const gitMatch = url.match(/^[^@]+@([^:]+):/);
  if (gitMatch) {
    return gitMatch[1] ?? null;
  }

  return null;
}

/**
 * Extracts unique SSH hosts from a list of repository URLs.
 */
export function extractUniqueSshHosts(urls: string[]): string[] {
  const hosts = new Set<string>();

  for (const url of urls) {
    const host = extractSshHost(url);
    if (host) {
      hosts.add(host);
    }
  }

  return Array.from(hosts);
}

/**
 * Checks if a host is already in the known_hosts file.
 */
export async function isHostKnown(host: string): Promise<boolean> {
  const knownHostsPath = join(homedir(), '.ssh', 'known_hosts');

  try {
    const content = await readFile(knownHostsPath, 'utf-8');
    // Check for host in various formats (plain, hashed entries won't match but that's ok)
    const hostPatterns = [
      new RegExp(`^${escapeRegex(host)}[,\\s]`, 'm'),
      new RegExp(`^\\[${escapeRegex(host)}\\]:\\d+[,\\s]`, 'm'),
    ];

    return hostPatterns.some((pattern) => pattern.test(content));
  } catch {
    // File doesn't exist or can't be read
    return false;
  }
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Adds a host's SSH key to known_hosts using ssh-keyscan.
 * Returns true if successful, false otherwise.
 */
export function addHostKey(host: string): boolean {
  const knownHostsPath = join(homedir(), '.ssh', 'known_hosts');

  // Use ssh-keyscan to fetch the host key and append to known_hosts
  const result = executeSync(`ssh-keyscan -H "${host}" >> "${knownHostsPath}" 2>/dev/null`, {
    cwd: process.cwd(),
  });

  return result.exitCode === 0;
}

/**
 * Ensures all SSH hosts for the given repository URLs are in known_hosts.
 * Prompts or automatically adds missing host keys.
 *
 * @param urls - Array of git repository URLs
 * @returns Object with arrays of added and failed hosts
 */
export async function ensureSshHostsKnown(
  urls: string[]
): Promise<{ added: string[]; failed: string[] }> {
  const hosts = extractUniqueSshHosts(urls);
  const added: string[] = [];
  const failed: string[] = [];

  if (hosts.length === 0) {
    return { added, failed };
  }

  for (const host of hosts) {
    const known = await isHostKnown(host);

    if (!known) {
      output.info(`Adding SSH host key for ${host}...`);
      const success = addHostKey(host);

      if (success) {
        output.success(`Added host key for ${host}`);
        added.push(host);
      } else {
        output.error(`Failed to add host key for ${host}`);
        failed.push(host);
      }
    }
  }

  return { added, failed };
}
