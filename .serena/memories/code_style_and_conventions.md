# Code Style and Conventions

## TypeScript Configuration

- **Strict mode** enabled with additional strict flags:
  - `noUncheckedIndexedAccess: true`
  - `noImplicitOverride: true`
  - `noPropertyAccessFromIndexSignature: true`
  - `exactOptionalPropertyTypes: true`
- Target: ES2022, Module: NodeNext, Lib: ES2024
- ESM only (`"type": "module"` in package.json)

## Import Conventions

- Use `node:` prefix for Node.js built-in imports (e.g., `import { readFile } from 'node:fs/promises'`)
- Use `.js` extension in relative imports (required by NodeNext module resolution)
- Barrel exports via `index.ts` files in subdirectories

## Naming Conventions

- **Functions**: camelCase (e.g., `readMetaConfig`, `createDefaultConfig`, `addProject`)
- **Classes**: PascalCase (e.g., `ConfigError`)
- **Constants**: UPPER_SNAKE_CASE for file-level constants (e.g., `META_FILE`, `LOOPRC_FILE`)
- **Zod schemas**: PascalCase with `Schema` suffix (e.g., `MetaConfigSchema`, `LoopRcSchema`)
- **Types/Interfaces**: PascalCase (e.g., `MetaConfig`, `ExecutorResult`, `FilterOptions`)
- **Unused variables**: Prefix with `_` (ESLint rule: `argsIgnorePattern: '^_'`, `varsIgnorePattern: '^_'`)

## Code Patterns

- Prefer `async/await` over callbacks
- Use Zod for runtime validation of external data (config files, CLI input)
- Use picocolors for terminal styling (NOT chalk)
- Keep functions pure where possible
- Custom error classes extend `Error` (e.g., `ConfigError`)
- Export functions individually (no default exports)

## ESLint

- Uses `@eslint/js` recommended + `typescript-eslint` recommended
- Ignores: `dist/`, `coverage/`, `node_modules/`

## Build

- tsup with ESM format, dts generation, sourcemaps, tree-shaking
- Entry points: `src/index.ts` (library) and `src/cli.ts` (CLI)
