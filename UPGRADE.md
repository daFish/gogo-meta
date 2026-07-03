# Upgrading to v3

gogo-meta v3 is a full rewrite of the tool in **Go**. Versions 1.x and 2.x were
written in TypeScript and distributed as the npm package `@dafish/gogo-meta`; v3
is a single self-contained binary with no Node.js or Bun runtime dependency.

The command-line interface and the `.gogo` configuration format are
**behavior-compatible** with v2 — your existing config files and scripts keep
working. Upgrading is mostly a matter of swapping how `gogo` is installed, plus a
few small behavior changes worth knowing about (see
[Behavior changes](#behavior-changes-in-v3)).

> **Version 2 is still usable, but no longer maintained.** It will not receive
> further bug fixes, security patches, or features. If you cannot migrate right
> now, v2 will keep running — see [Staying on v2](#staying-on-v2) — but plan to
> move to v3.

---

## At a glance

| | v2 (TypeScript) | v3 (Go) |
| --- | --- | --- |
| Runtime | Node.js / Bun | none (native binary) |
| Distribution | npm (`@dafish/gogo-meta`) | Homebrew cask, GitHub release binaries, Docker |
| Config format | `.gogo` / `.gogo.yaml` / `.gogo.yml` | **unchanged** |
| CLI commands | `init`, `exec`, `run`, `git …`, `project …`, `npm …`, … | **unchanged** |

---

## Step 1 — Remove the old (v2) version

v2 was installed as a global npm package, so uninstall it with the same package
manager you used to install it.

First, find where the current `gogo` comes from:

```bash
which gogo
gogo --version   # v2 prints a semver like 2.0.0
```

Then remove it:

```bash
# npm
npm uninstall -g @dafish/gogo-meta

# bun
bun remove -g @dafish/gogo-meta

# pnpm
pnpm remove -g @dafish/gogo-meta

# yarn (v1)
yarn global remove @dafish/gogo-meta
```

Confirm it is gone:

```bash
which gogo   # should print nothing (or error) before you install v3
```

If `which gogo` still resolves after uninstalling, an old binary or shell alias
is lingering — remove the stray file or `unalias gogo` before continuing so the
new install is the one that runs.

---

## Step 2 — Install v3

Pick **one** of the following.

### Homebrew (macOS — recommended)

```bash
brew install --cask daFish/gogo-meta/gogo
```

This taps `daFish/homebrew-gogo-meta` and installs the `gogo` cask. It also clears
the macOS quarantine flag for you, so there is no Gatekeeper prompt on first run.
To upgrade later:

```bash
brew upgrade --cask gogo
```

The cask is macOS only (Monterey or newer). On Linux, use the pre-built binary or
Docker below.

### Pre-built binary

Download the archive for your platform from the
[latest release](https://github.com/daFish/gogo-meta/releases), extract it, and
put the `gogo` binary on your `$PATH`:

```bash
# Example — adjust version, OS (darwin/linux) and arch (amd64/arm64)
tar -xzf gogo_3.0.0_darwin_arm64.tar.gz
sudo mv gogo /usr/local/bin/
```

> **macOS note:** release binaries are ad-hoc signed but not notarized, so
> Gatekeeper may block the binary on first run when you download it manually. The
> Homebrew cask strips the quarantine flag for you automatically; if you install
> the raw binary yourself, clear it with:
>
> ```bash
> xattr -d com.apple.quarantine /usr/local/bin/gogo
> ```

### Docker

```bash
docker pull ghcr.io/dafish/gogo-meta
```

Run it with your working directory and SSH keys mounted:

```bash
docker run -it --rm \
  -v "$PWD":/app \
  -v "$HOME/.ssh":/root/.ssh:ro \
  -w /app \
  ghcr.io/dafish/gogo-meta <command>
```

### From source

Requires Go 1.24+:

```bash
git clone https://github.com/daFish/gogo-meta.git
cd gogo-meta
make build
sudo mv dist/gogo /usr/local/bin/
```

---

## Step 3 — Verify

```bash
gogo --version   # should print 3.x
gogo validate    # sanity-check your existing .gogo config
```

Your `.gogo`, `.gogo.yaml`, and `.gogo.yml` files require **no changes**. Run
`gogo validate` to confirm they parse cleanly under v3.

---

## Behavior changes in v3

The rewrite is intentionally behavior-compatible for all valid inputs. A few
deliberate divergences harden security and robustness — these are the only
things that might affect an existing workflow:

- **Built-in commands run without a shell.** Internal `git` / `npm` /
  `ssh-keyscan` calls now execute as argument vectors instead of being
  interpolated into a `/bin/sh -c` string. This closes a command-injection hole
  when cloning an untrusted meta repository. `gogo exec` and `gogo run` still run
  your commands through a shell — that is their purpose — so shell features
  (pipes, globs, `&&`) in `exec`/`run` are unaffected.
- **Project paths are validated.** Project keys in `.gogo` (and the `folder`
  argument of `gogo project create` / `gogo project import`) must be relative and
  stay inside the repository. Absolute paths and `..` traversal are now rejected.
  If a config used an absolute or escaping path, fix it to a relative in-repo
  path.
- **`gogo init` no longer accepts `-f` as a shorthand for `--force`.** Use the
  long form:

  ```bash
  gogo init --force
  ```

  The global `-f` / `--file` overlay flag is unchanged and works exactly as in
  v2.

### Coming from v1.x?

If you are jumping straight from v1.x (skipping v2), note the one breaking change
introduced in **v2**: legacy **`.looprc` files are no longer read**. Move any
directory-exclusion entries into the `--exclude-only` / `--exclude-pattern`
filters, or remove them from the `projects` map in `.gogo`. `gogo validate` also
no longer inspects `.looprc`.

---

## Staying on v2

v2 remains installable and functional; it is simply frozen. To keep or reinstall
it:

```bash
npm install -g @dafish/gogo-meta@2   # or the bun/pnpm/yarn equivalent
```

Because v2 receives no further updates, this is a stopgap only. When you are
ready, follow [Step 1](#step-1--remove-the-old-v2-version) and
[Step 2](#step-2--install-v3) to move to the maintained v3 release.

---

## Troubleshooting

- **`gogo --version` still shows a 2.x version after installing v3.** An old
  install is still first on your `$PATH`. Run `which -a gogo` to list every
  `gogo` on the path and remove the stale one (or fix the ordering).
- **macOS says the binary is "damaged" or "cannot be opened".** You installed the
  raw binary manually; clear the quarantine flag as shown in
  [Step 2](#pre-built-binary), or install via Homebrew instead.
- **`gogo validate` reports an invalid project path.** A project key or imported
  folder uses an absolute path or `..`. Rewrite it as a relative path inside the
  meta repository (see [Behavior changes](#behavior-changes-in-v3)).
