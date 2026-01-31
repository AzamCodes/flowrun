# Flowrun CLI

Flowrun is a professional, cross-platform workflow automation tool for developers. It executes steps defined in YAML files with capability for complex dependency graphs (DAGs), parallel execution, and strict reliability.

## Features

- **DAG Execution**: Define dependencies between steps using `needs`.
- **Parallelism**: Run independent steps concurrently with `parallel: true`.
- **Robustness**: Fail-fast architecture, timeouts, and automatic retries.
- **Environment Management**: Hierarchical env vars (System < CLI < Workflow < Step).
- **Observability**: Structured logging (Text/JSON) and execution summaries.
- **Developer Experience**:
  - `init` command to bootstrap workflows.
  - `--dry-run` to validate logic.
  - `--steps` and `--tags` for targeted execution.

## Installation

### From Source
```bash
go install flowrun
```

### Pre-built Binaries
Download the latest release for Linux, macOS, or Windows from the [Releases](https://github.com/yourusername/flowrun/releases) page.

## Quick Start

1. **Initialize a Workflow**
   ```bash
   flowrun init my-workflow.yaml
   ```

2. **Execute**
   ```bash
   flowrun run my-workflow.yaml
   ```

## Workflow Syntax

```yaml
name: Production Deployment
fail_fast: true
env:
  AppEnv: production
required_env:
  - API_KEY

steps:
  - name: Build
    run: make build
    timeout: 5m
    parallel: true

  - name: Lint
    run: make lint
    parallel: true

  - name: Test
    run: make test
    needs: [Build]
    retry: 2

  - name: Deploy
    run: ./deploy.sh
    needs: [Test, Lint]
    tags: [release]
```

## Dependencies & Parallelism

Flowrun supports Directed Acyclic Graphs (DAGs). 

- **Sequential (Default)**: Steps run one by one in order.
- **Dependencies**: Use `needs: [Step Name]` to enforce order. If a dependency fails, the step is skipped.
- **Parallelism**: Set `parallel: true` on steps. They will run concurrently up to the limit set by `--max-parallel` (default: CPU cores) once their dependencies are met.

## CLI Reference

### `flowrun run <file>`

| Flag | Description |
|------|-------------|
| `--max-parallel`| Maximum number of parallel steps (default: NumCPU). |
| `--dry-run` | Print execution plan without running commands. |
| `--steps` | Comma-separated list of steps to explicitly run. |
| `--tags` | Comma-separated list of tags to run. |
| `--env` | Set environment variable (e.g. `-e KEY=VAL`). |
| `--env-file` | Load environment variables from a `.env` file. |
| `--log-level` | Set log level (DEBUG, INFO, WARN, ERROR). |
| `--json` | Output logs in JSON format. |

### `flowrun init <file>`
Generates a new workflow template.

## Exit Codes

- `0`: Success (all steps succeeded or were skipped safely).
- `1`: Runtime Error (a step failed).
- `2`: Validation Error (syntax, missing dependency, circular dependency).

---

## License
MIT
