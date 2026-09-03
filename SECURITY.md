# Security Policy

## Reporting a vulnerability

Please report suspected security issues privately using GitHub's
[private vulnerability reporting](https://github.com/daFish/gogo-meta/security/advisories/new)
(the **Report a vulnerability** button under the repository's **Security** tab).
Please do not open a public issue for security reports.

## Trust model

gogo runs commands across many repositories on your behalf. Two commands run
**arbitrary shell commands** by design:

- `gogo exec "<command>"` runs the command you pass.
- `gogo run <name>` runs a command defined in the `commands` section of the
  `.gogo` file.

Because `gogo run` executes whatever a project's `.gogo` defines, **running it
in a repository means trusting that repository's authors** — the same trust you
extend to `npm run`, a `Makefile`, or a git hook. Only run gogo in
meta-repositories you trust, and review a cloned or shared `.gogo` (its
`commands` and `projects`) before running `gogo run`, `gogo git update`, or
`gogo git clone`.

## What gogo does to reduce risk

- **Config discovery is ownership-checked.** gogo searches parent directories
  for a `.gogo`, but refuses one that is not owned by the current user, so it
  does not silently adopt a config planted by another user in a shared parent
  directory (for example `/tmp`).
- **SSH host keys are never added automatically.** gogo warns about unknown
  hosts and leaves host-key verification to you (or to `ssh`'s own prompt); it
  does not run `ssh-keyscan` and trust whatever key the network returns.
- **Git URLs are validated.** URLs from the config and CLI are rejected if they
  begin with `-` or use a remote-helper transport such as `ext::`, which git
  would otherwise execute as a command.
- **Local npm links stay inside the project.** `gogo npm link --all` will not
  follow a crafted dependency name or a symlinked `node_modules` out of the
  repository.
- **Child output is sanitized.** Control characters and escape sequences in a
  command's output cannot forge gogo's own status lines.

## What gogo does not protect against

These protections stop another party from injecting a config or command into
your session. They do **not** vet the contents of a config you choose to run: a
`.gogo` you own, or one inside a repository you cloned, can still define
`commands` that run with your privileges. Review before you run.

## Running the container

The published image runs as root so it can operate on mounted working copies.
When mounting a repository you own, run the container as your own user so gogo
does not act as root and file ownership stays intact:

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" -w /work \
  ghcr.io/dafish/gogo-meta:latest git status
```
