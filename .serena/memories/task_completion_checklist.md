# Task Completion Checklist

When a coding task is completed, run the following checks:

1. **Type checking**: `bun run typecheck`
   - Ensures no TypeScript errors (strict mode with extra flags)

2. **Linting**: `bun run lint`
   - ESLint with typescript-eslint recommended rules

3. **Unit tests**: `bun run test:unit`
   - Fast feedback, uses memfs for filesystem mocking

4. **Integration tests**: `bun run test:integration`
   - Verifies command behavior end-to-end

5. **All tests**: `bun run test`
   - Full test suite (unit + integration + e2e)

## Quick Check (minimum before committing)

```bash
bun run typecheck && bun run lint && bun run test
```
