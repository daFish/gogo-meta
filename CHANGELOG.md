# [2.0.0](https://github.com/daFish/gogo-meta/compare/v1.8.0...v2.0.0) (2026-05-30)


* refactor!: remove legacy .looprc support ([fc0102d](https://github.com/daFish/gogo-meta/commit/fc0102dca6f07b5529e8943e2d1298502c806310))


### BREAKING CHANGES

* `.looprc` is no longer read. A repository that relied on a
`.looprc` ignore list to exclude directories from `exec`/`run`/`git`/`npm` will
now run those commands against the previously-excluded directories. Replace it
with the `--exclude-only`/`--exclude-pattern` filters, or remove the entries
from the `projects` map in `.gogo`. `gogo validate` also no longer validates
`.looprc` files.

# [1.8.0](https://github.com/daFish/gogo-meta/compare/v1.7.0...v1.8.0) (2026-05-30)


### Features

* **migrate:** keep .gitignore in sync when moving repos ([f2522b7](https://github.com/daFish/gogo-meta/commit/f2522b77f6d99a54496dffb55e4756fa13b86112))
* **migrate:** reconcile the working copy with the configuration ([8e8c05f](https://github.com/daFish/gogo-meta/commit/8e8c05f09235dc7af2e01aefff1f701add8555ac))

# [1.7.0](https://github.com/daFish/gogo-meta/compare/v1.6.0...v1.7.0) (2026-05-30)


### Features

* **validate:** check configured projects exist in the working copy ([487786e](https://github.com/daFish/gogo-meta/commit/487786ec7e3608c7a27fd6914c3d0b113ad1a971))

# [1.6.0](https://github.com/daFish/gogo-meta/compare/v1.5.0...v1.6.0) (2026-03-06)


### Features

* add option to validate configuration files ([53353ea](https://github.com/daFish/gogo-meta/commit/53353ea0b4a02a73722442c782171af032386323))
* display a list of failed projects after git pull ([02394a0](https://github.com/daFish/gogo-meta/commit/02394a00e1a11274ad981fa827965f148f1dd1eb))

# [1.5.0](https://github.com/daFish/gogo-meta/compare/v1.4.0...v1.5.0) (2026-03-05)


### Bug Fixes

* **project:** pass empty overlay list on write to prevent absorption ([b637bf0](https://github.com/daFish/gogo-meta/commit/b637bf01a67fde0855e65894f918f458490e3cea))


### Features

* **cli:** add -f/--file global option for overlay configs ([02ef2ca](https://github.com/daFish/gogo-meta/commit/02ef2ca77e752a1b6df90896c04c6cafed05485d))
* **config:** add overlay config merging and multi-file support ([d1335c1](https://github.com/daFish/gogo-meta/commit/d1335c14e9d11a400a69e9b1fc1ecfc3a8ae53f3))

# [1.4.0](https://github.com/daFish/gogo-meta/compare/v1.3.0...v1.4.0) (2026-03-03)


### Features

* **config:** add YAML support for .gogo configuration files ([d298ebc](https://github.com/daFish/gogo-meta/commit/d298ebcfc0f958bf821cbe573483bc4b907f6411))
* **init:** add --format flag to choose JSON or YAML config ([c2117bb](https://github.com/daFish/gogo-meta/commit/c2117bba37bb1f4e2ba56d6d95362cc59e8bafe3))

# [1.3.0](https://github.com/daFish/gogo-meta/compare/v1.2.0...v1.3.0) (2026-02-12)


### Features

* build a container image if a new release was published ([eb055e1](https://github.com/daFish/gogo-meta/commit/eb055e1aa05638e97b0c1c90793f80f07faa233b))

# [1.2.0](https://github.com/daFish/gogo-meta/compare/v1.1.1...v1.2.0) (2026-02-12)


### Features

* add imported projects to gitignore ([e56f38e](https://github.com/daFish/gogo-meta/commit/e56f38e59064de59a6237d9904790e489fde7605))

## [3.0.1](https://github.com/daFish/gogo-meta/compare/v3.0.0...v3.0.1) (2026-07-21)


### Documentation

* add v3 upgrade guide ([2ca51da](https://github.com/daFish/gogo-meta/commit/2ca51dab4fb8b32b4c4021951fda16d9e6f4ee6e))
* link upgrade guide from README ([818f789](https://github.com/daFish/gogo-meta/commit/818f789195cb4af1164b928c150c39bd9855ff66))

## [3.0.0](https://github.com/daFish/gogo-meta/compare/v2.0.0...v3.0.0) (2026-07-03)


### ⚠ BREAKING CHANGES

* rewrite the tool in Go (until 6ae349af)

### Bug Fixes

* **ci:** match release-please tags to existing v-prefixed history ([e0b777f](https://github.com/daFish/gogo-meta/commit/e0b777fb084450ff6f189b2c67a94bcec3c14806))
* **deps:** update dependency commander to v15 ([e1ac724](https://github.com/daFish/gogo-meta/commit/e1ac7242c75b6316aca54a40990f76279522a88a))


### Code Refactoring

* clean up the rewrite (dead code, typed Loop, ssh DI, parallel fix) ([d7f0071](https://github.com/daFish/gogo-meta/commit/d7f00713df38043f707930642ab0aec5b68570a4))
* implement recent changes in Go (until 44344a19) ([a243633](https://github.com/daFish/gogo-meta/commit/a2436336651025f36040a3d94f7ba0e2e85d8d1e))
* rewrite the tool in Go (until 6ae349af) ([6a33235](https://github.com/daFish/gogo-meta/commit/6a33235f409e18035aba5258aad7d1a71fe80ee7))
* **security:** use shell-free built-ins and improve signal handling (diverges from TS version) ([4e9ebee](https://github.com/daFish/gogo-meta/commit/4e9ebeed1588a2067ba332e8fd40335053de0865))

## [1.1.1](https://github.com/daFish/gogo-meta/compare/v1.1.0...v1.1.1) (2026-01-27)


### Bug Fixes

* add SSH host key verification before cloning repositories ([3aba347](https://github.com/daFish/gogo-meta/commit/3aba3479ccad8ee2bce6f5a13c36117d3a110bb5)), closes [#12](https://github.com/daFish/gogo-meta/issues/12)

# [1.1.0](https://github.com/daFish/gogo-meta/compare/v1.0.0...v1.1.0) (2026-01-19)


### Features

* add option --no-clone when importing projects ([450ef99](https://github.com/daFish/gogo-meta/commit/450ef99a22b4d89ffcd8a3365e3ddff540950064))

# 1.0.0 (2026-01-12)


### Bug Fixes

* add helper function to fix build errors on different platforms ([2293228](https://github.com/daFish/gogo-meta/commit/22932284c8f509f9ea135eb4a0004c1f1724a7e3))
* **deps:** update dependency commander to v14 ([717ebd0](https://github.com/daFish/gogo-meta/commit/717ebd09e2aeb4c1ca4e170a6cc388e2bb59e29a))
* **deps:** update dependency zod to v4 ([f8541e7](https://github.com/daFish/gogo-meta/commit/f8541e73001f1860b65d0ede64186999d968ef3d))
* handle timeouts on linux platforms better ([670dbbb](https://github.com/daFish/gogo-meta/commit/670dbbbd394090cd97e2768ea8a778f199cd7b5d))


### Features

* add semantic release ([80d32dc](https://github.com/daFish/gogo-meta/commit/80d32dcfb03dcaf5d192fda8559ede56ef2328c0))
* add support for pre-defined commands ([a634273](https://github.com/daFish/gogo-meta/commit/a634273b296f29485a1ce4fefe710fdc995cc11f))
* initial commit ([f2dfb85](https://github.com/daFish/gogo-meta/commit/f2dfb859dbd1717cd3dfda316b156d3328a8d758))
