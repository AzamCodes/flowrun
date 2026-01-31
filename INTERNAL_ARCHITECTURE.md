# Internal Architecture

## Core Components

### 1. Workflow Loader (`internal/workflow`)
Parses YAML into `Workflow` and `Step` structs.
- Validates fields.
- Checks for circular dependencies (DAG cycle detection).
- Flattens environment variables.

### 2. Executor (`internal/executor`)
The engine of Flowrun.
- **State Machine**: Uses `stepStatus` (Pending, Running, Success, Failed, Skipped, Cached) and `sync.Cond` for event-driven scheduling.
- **Scheduler**: A loop that monitors step states and launches steps whose dependencies are met.
- **Concurrency**: Manages a worker pool (`max-parallel`) using a semaphore channel.
- **Context**: Propagates cancellation on failure (Fail-Fast).

### 3. Caching (`internal/cache`)
File-based persistence (`.flowrun_cache.json`).
- **Hash Algorithm**: `SHA256(Workflow + Step + Command + EffectiveEnv + DependencyHashes)`.
- **Merkle Tree**: Including dependency hashes ensures downstream steps re-run if upstream changes.

### 4. Logger (`internal/logger`)
Wrapper around `log/slog`.
- formatting (Text/JSON).
- Levels (DEBUG, INFO, ERROR).

## Execution Flow

1. CLI parses flags (`--use-cache`, `--max-parallel`).
2. `Load` workflow.
3. `Execute`:
   - Merge Env.
   - Init Cache.
   - Loop:
     - Check Dependencies.
     - Compute Hash.
     - Check Cache -> If Hit, Update State to Cached.
     - Else -> Launch Runner Goroutine.
   - Wait for completion.
   - Print Summary.
