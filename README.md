# polyci

[![CI](https://github.com/Arekkusul/polyci/actions/workflows/ci.yml/badge.svg)](https://github.com/Arekkusul/polyci/actions/workflows/ci.yml)

**One CI gate for every stack.** A single, zero-dependency Go binary that detects a project's
language(s) and runs the right checks — `fmt → lint → typecheck → build → test` — the same way
locally and in CI. Instead of hand-writing a bespoke `ci.yml` per repo, point polyci at any repo
and it does the correct thing.

Built to run uniformly across a polyglot set of projects (Python, Go, Rust, Swift, C++, Node).

## Install

```bash
go install github.com/Arekkusul/polyci@latest   # needs Go 1.26+
```

## Use

```bash
polyci detect      # which stack(s) is this? -> e.g. "python"
polyci list        # show the exact gate commands that would run
polyci run         # run them; exits non-zero if any gate fails
polyci run --only test          # run just one gate
polyci run --skip clippy        # drop a gate
polyci run --keep-going         # run all gates even after a failure
polyci init        # write a GitHub Actions workflow tailored to this repo
```

`--dir DIR` points any command at another directory (default: the current one).

## What it runs per stack

Detection is by marker file; a repo can match more than one (polyglot repos run each).

| Stack | Marker | Default gates |
|---|---|---|
| Go | `go.mod` | `gofmt -l .` · `go vet ./...` · `go build ./...` · `go test ./...` |
| Rust | `Cargo.toml` | `cargo fmt --check` · `cargo clippy -D warnings` · `cargo build` · `cargo test` |
| Python | `pyproject.toml` … | `ruff check .`¹ · `mypy <pkg>`² · `pytest` (via the project's venv if present) |
| Node | `package.json` | `npm ci`/`install` · the `lint`/`typecheck`/`build`/`test` scripts you define |
| Swift | `Package.swift` | `swift build` (add your test command via `.polyci.json`) |
| C++ | `CMakeLists.txt` | `cmake -S . -B build` · `cmake --build build` · `ctest` |

¹ only when `pyproject.toml` has a `[tool.ruff]` section.
² only when `[tool.mypy]` is present (target auto-detected: the first package dir with `__init__.py`) or a `mypy_target` is set in config.

## Per-project tweaks — `.polyci.json` (optional)

Drop a `.polyci.json` in the repo root to absorb quirks without changing the tool:

```json
{
  "env": { "DYLD_LIBRARY_PATH": "/opt/homebrew/opt/expat/lib" },
  "python_bin": ".venv/bin/python",
  "mypy_target": "app",
  "skip": ["clippy"],
  "only": ["test"],
  "override": { "test": ["swift", "run", "crosspostkit-tests"] },
  "steps": [ { "name": "smoke", "command": ["bash", "scripts/smoke.sh"] } ]
}
```

- **env** — extra environment for every step (e.g. this Mac's pyexpat workaround).
- **python_bin** — interpreter for the Python gates (default: `.venv/bin/python` if it exists, else `python3`).
- **mypy_target** — enable mypy against this path.
- **skip / only** — drop or restrict steps by name (also available as `--skip` / `--only`).
- **override** — replace a step's command (e.g. Swift's executable test target).
- **steps** — extra steps appended after the defaults (e.g. a browser smoke test).

## In CI

**Tailored, per-repo:** `polyci init` inspects the repo and writes `.github/workflows/ci.yml`
with exactly the toolchains it needs, then installs polyci and runs it.

**Shared, across many repos:** call the reusable workflow so every repo's CI is one block:

```yaml
name: CI
on: [push, pull_request]
jobs:
  ci:
    uses: Arekkusul/polyci/.github/workflows/reusable.yml@main
    with:
      python: true    # set the toolchains this repo needs
```

> Installing polyci in CI uses `go install`. For a **private** polyci repo, set `GOPRIVATE`
> and provide a token, or vendor a prebuilt binary.

## Layout

```
main.go              CLI (detect/list/run/init/version)
workflow.go          tailored GitHub Actions generator
internal/detect      stack detection from marker files
internal/config      .polyci.json loader
internal/plan        default pipelines + config overrides
internal/run         step executor (streaming, timing, exit code)
```

Its own CI dogfoods it: the workflow builds polyci and runs `./polyci run` on itself.
