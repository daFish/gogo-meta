import { join } from 'node:path';
import { readFile, symlink, unlink, mkdir } from 'node:fs/promises';
import { readMetaConfig, getMetaDir, fileExists } from '../../core/config.js';
import { applyFilters, createFilterOptions } from '../../core/filter.js';
import * as output from '../../core/output.js';

interface LinkOptions {
  includeOnly?: string;
  excludeOnly?: string;
  includePattern?: string;
  excludePattern?: string;
  all?: boolean;
}

interface PackageJson {
  name?: string;
  dependencies?: Record<string, string>;
  devDependencies?: Record<string, string>;
}

async function readPackageJson(dir: string): Promise<PackageJson | null> {
  const packageJsonPath = join(dir, 'package.json');
  if (!(await fileExists(packageJsonPath))) {
    return null;
  }

  try {
    const content = await readFile(packageJsonPath, 'utf-8');
    return JSON.parse(content) as PackageJson;
  } catch {
    return null;
  }
}

async function createSymlink(target: string, linkPath: string): Promise<boolean> {
  try {
    if (await fileExists(linkPath)) {
      await unlink(linkPath);
    }

    const parentDir = join(linkPath, '..');
    await mkdir(parentDir, { recursive: true });

    await symlink(target, linkPath, 'dir');
    return true;
  } catch (error) {
    output.error(`Failed to create symlink: ${linkPath} -> ${target}`);
    return false;
  }
}

export async function linkCommand(options: LinkOptions = {}): Promise<void> {
  const cwd = process.cwd();
  const metaDir = await getMetaDir(cwd);

  if (!metaDir) {
    throw new Error('Not in a gogo-meta repository. Run "gogo init" first.');
  }

  const config = await readMetaConfig(cwd);
  const filterOptions = createFilterOptions(options);

  let projectPaths = Object.keys(config.projects);
  projectPaths = applyFilters(projectPaths, filterOptions);

  if (projectPaths.length === 0) {
    output.warning('No projects match the specified filters');
    return;
  }

  const projectPackages: Map<string, { path: string; packageJson: PackageJson }> = new Map();

  for (const projectPath of projectPaths) {
    const fullPath = join(metaDir, projectPath);
    const packageJson = await readPackageJson(fullPath);

    if (packageJson?.name) {
      projectPackages.set(packageJson.name, { path: fullPath, packageJson });
    }
  }

  if (projectPackages.size === 0) {
    output.warning('No projects with package.json found');
    return;
  }

  output.info(`Found ${projectPackages.size} linkable projects`);

  let linkCount = 0;

  if (options.all) {
    for (const [consumerName, consumer] of projectPackages) {
      const allDeps = {
        ...consumer.packageJson.dependencies,
        ...consumer.packageJson.devDependencies,
      };

      for (const [depName] of Object.entries(allDeps)) {
        const provider = projectPackages.get(depName);
        if (provider) {
          const nodeModulesPath = join(consumer.path, 'node_modules', depName);
          const success = await createSymlink(provider.path, nodeModulesPath);

          if (success) {
            output.projectStatus(
              `${consumerName}`,
              'success',
              `linked ${depName}`
            );
            linkCount++;
          }
        }
      }
    }
  } else {
    for (const [packageName, info] of projectPackages) {
      output.info(`Creating global link for ${packageName}...`);
      const { execute } = await import('../../core/executor.js');
      const result = await execute('npm link', { cwd: info.path });

      if (result.exitCode === 0) {
        output.projectStatus(packageName, 'success', 'linked globally');
        linkCount++;
      } else {
        output.projectStatus(packageName, 'error', result.stderr);
      }
    }
  }

  output.success(`Created ${linkCount} links`);
}
