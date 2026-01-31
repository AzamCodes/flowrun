package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"flowrun/internal/cache"
	"flowrun/internal/logger"
	"flowrun/internal/workflow"
)

// Options allows configuring the execution environment.
type Options struct {
	Stdout      io.Writer
	Stderr      io.Writer
	DryRun      bool
	Env         map[string]string
	MaxParallel int
	UseCache    bool
	Force       bool
}

// Execute runs the workflow steps sequentially.
// It stops at the first failure.


type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusSuccess
	statusFailed
	statusSkipped
	statusCached
)

type executionState struct {
	mu     sync.Mutex
	status map[string]stepStatus
	cond   *sync.Cond
}

func newExecutionState(steps []workflow.Step) *executionState {
	s := &executionState{
		status: make(map[string]stepStatus),
	}
	s.cond = sync.NewCond(&s.mu)
	for _, step := range steps {
		s.status[step.Name] = statusPending
	}
	return s
}

// Execute runs the workflow steps based on dependencies.
func Execute(ctx context.Context, wf *workflow.Workflow, opts *Options) error {
	logger.Info("Workflow execution started", "workflow", wf.Name, "parallelism", opts.MaxParallel, "fail_fast", wf.IsFailFast(), "use_cache", opts.UseCache)
	startTime := time.Now()

	// 1. Merge global environment variables
	globalEnv := mergeEnv(os.Environ(), wf.Env, opts.Env)

	// 2. Validate required environment variables
	if err := validateRequiredEnv(wf.RequiredEnv, globalEnv); err != nil {
		return err
	}

	state := newExecutionState(wf.Steps)
	
	// Initialize Cache Store
	var cacheStore *cache.Store
	if opts.UseCache {
		var err error
		cacheStore, err = cache.New("")
		if err != nil {
			logger.Warn("Failed to load cache, proceeding without cache", "error", err)
		}
	}
	
	// Track hashes of steps to support dependency hashing
	stepHashes := make(map[string]string)
	var hashesMu sync.RWMutex

	// Create a context that can be cancelled on first failure if fail-fast is enabled
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Worker pool control
	maxWorkers := opts.MaxParallel
	if maxWorkers < 1 {
		maxWorkers = runtime.NumCPU()
	}
	sem := make(chan struct{}, maxWorkers)
	
	// Track active workers to know when to exit
	var wg sync.WaitGroup
	
	// Errors channel
	errChan := make(chan error, len(wf.Steps))

	// Map for easy access
	stepMap := make(map[string]workflow.Step)
	for _, s := range wf.Steps {
		stepMap[s.Name] = s
	}

	for {
		state.mu.Lock()
		
		// Check if all done
		allDone := true
		for _, s := range wf.Steps {
			st := state.status[s.Name]
			if st == statusPending || st == statusRunning {
				allDone = false
				break
			}
		}
		
		if allDone {
			state.mu.Unlock()
			break
		}

		// Find runnable steps
		var runnable []workflow.Step
		progress := false // Track if we updated any state (e.g. skipped steps)

		for _, s := range wf.Steps {
			if state.status[s.Name] != statusPending {
				continue
			}

			// Check dependencies
			ready := true
			failedDep := false

			for _, depName := range s.Needs {
				depStatus := state.status[depName]
				if depStatus != statusSuccess && depStatus != statusCached {
					ready = false
					// Cached steps count as success for dependency purposes
					if depStatus == statusFailed || depStatus == statusSkipped {
						failedDep = true
					}
					break
				}
			}

			if failedDep {
				state.status[s.Name] = statusSkipped
				logger.Info("Skipping step due to failed dependency", "step", s.Name)
				progress = true
				continue
			}

			if ready {
				// Check concurrency rules
				runningCount := 0
				for _, st := range state.status {
					if st == statusRunning {
						runningCount++
					}
				}
				
				if s.Parallel {
					runnable = append(runnable, s)
				} else {
					if runningCount == 0 {
						runnable = append(runnable, s)
					}
				}
			}
		}
		
		// If we found runnable steps, launch them
		if len(runnable) > 0 {
			launched := false
			for _, s := range runnable {
				if !s.Parallel {
					// Exclusive
					if !launched {
						state.status[s.Name] = statusRunning
						wg.Add(1)
						go func(step workflow.Step) {
							defer wg.Done()
							runStepWrapper(execCtx, wf.Name, step, opts, globalEnv, state, stepMap, sem, errChan, wf.IsFailFast(), cancel, cacheStore, stepHashes, &hashesMu)
						}(s)
						launched = true
						progress = true
						break // One exclusive step at a time
					}
				} else {
					// Parallel
					state.status[s.Name] = statusRunning
					wg.Add(1)
					go func(step workflow.Step) {
						defer wg.Done()
						runStepWrapper(execCtx, wf.Name, step, opts, globalEnv, state, stepMap, sem, errChan, wf.IsFailFast(), cancel, cacheStore, stepHashes, &hashesMu)
					}(s)
					launched = true
					progress = true
				}
			}
		}

		if progress {
			// If we made progress (launched or skipped), don't wait. Re-evaluate.
			state.mu.Unlock()
			continue
		}

		// Wait for change
		state.cond.Wait()
		state.mu.Unlock()
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	totalDuration := time.Since(startTime)
	
	// Final Summary
	printSummary(wf, state)

	if len(errs) > 0 {
		logger.Error("Workflow failed", "errors", len(errs))
		return errs[0] // Return first error
	}

	logger.Info("Workflow completed successfully", "duration", totalDuration.String())
	return nil
}

func runStepWrapper(ctx context.Context, wfName string, step workflow.Step, opts *Options, globalEnv []string, state *executionState, stepMap map[string]workflow.Step, sem chan struct{}, errChan chan error, failFast bool, cancel context.CancelFunc, cacheStore *cache.Store, stepHashes map[string]string, hashesMu *sync.RWMutex) {
	// Acquire semaphore
	sem <- struct{}{}
	defer func() { <-sem }()

	// Signal on completion
	defer func() {
		state.mu.Lock()
		state.cond.Broadcast()
		state.mu.Unlock()
	}()
	
	// Check context
	if ctx.Err() != nil {
		state.mu.Lock()
		state.status[step.Name] = statusSkipped // Execute cancelled
		state.mu.Unlock()
		return
	}

	if opts.DryRun {
		logger.Info("[DRY-RUN] Step", "step", step.Name)
		state.mu.Lock()
		state.status[step.Name] = statusSuccess
		state.mu.Unlock()
		return
	}

	// Prepare environment and Cache Hash
	stepEnv := mergeEnv(globalEnv, step.Env)
	
	var currentHash string
	if cacheStore != nil {
		// Collect dependency hashes
		hashesMu.RLock()
		depHashes := make(map[string]string)
		for _, dep := range step.Needs {
			depHashes[dep] = stepHashes[dep]
		}
		hashesMu.RUnlock()

		currentHash = cache.ComputeHash(wfName, step, stepEnv, depHashes)

		// Check Cache
		if !opts.Force {
			if entry, hit := cacheStore.Get(wfName, step.Name); hit {
				if entry.Hash == currentHash && entry.ExitCode == 0 {
					logger.Info("Cache Hit - Skipping step", "step", step.Name)
					
					state.mu.Lock()
					state.status[step.Name] = statusCached
					state.mu.Unlock()
					
					// Store hash for dependents
					hashesMu.Lock()
					stepHashes[step.Name] = currentHash
					hashesMu.Unlock()
					return
				}
			}
		}
	}

	// Calculate index? We don't have linear index in DAG easily.
	// Step Retry Logic
	err := executeStepWithRetry(ctx, step, opts, stepEnv)
	
	state.mu.Lock()
	if err != nil {
		state.status[step.Name] = statusFailed
		if failFast {
			cancel()
		}
		errChan <- err
	} else {
		state.status[step.Name] = statusSuccess
		
		// Update Cache on success
		if cacheStore != nil && currentHash != "" {
			cacheStore.Set(wfName, step.Name, currentHash, 0)
			if err := cacheStore.Save(); err != nil {
				logger.Warn("Failed to save cache", "error", err)
			} else {
				// Store hash for dependents
				// Note: We need to do this outside of state lock to avoid deadlock scenarios?
				// No, stepHashes has its own lock.
				// But we are inside state lock currently.
				// This is fine as hashesMu is leaf lock.
			}
		}
	}
	state.mu.Unlock()
	
	// Store hash for dependents (needs to happen even if not cached, if we computed it)
	if cacheStore != nil && currentHash != "" && err == nil {
		hashesMu.Lock()
		stepHashes[step.Name] = currentHash
		hashesMu.Unlock()
	}
}

// executeStepWithRetry runs the step logic including timeouts and retries
func executeStepWithRetry(ctx context.Context, step workflow.Step, opts *Options, env []string) error {
	maxRetries := step.Retry
	for attempt := 0; attempt <= maxRetries; attempt++ {
		stepStart := time.Now()
		
		logger.Info("Starting step", "step", step.Name, "attempt", attempt+1)

		stepCtx := ctx
		if step.Timeout != "" {
			d, err := time.ParseDuration(step.Timeout)
			if err == nil {
				var cancelCtx context.CancelFunc
				stepCtx, cancelCtx = context.WithTimeout(ctx, d)
				defer cancelCtx()
			}
		}

		err := executeStep(stepCtx, step, opts, env)
		duration := time.Since(stepStart)

		if err == nil {
			logger.Info("Step completed", "step", step.Name, "duration", duration.String())
			return nil
		}

		logger.Error("Step execution failed", "step", step.Name, "error", err, "duration", duration.String())

		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				// retry
			}
		}
	}

	return fmt.Errorf("max retries exceeded")
}

func printSummary(wf *workflow.Workflow, state *executionState) {
	fmt.Println("\nWorkflow Summary:")
	state.mu.Lock()
	defer state.mu.Unlock()
	
	for _, step := range wf.Steps {
		st := state.status[step.Name]
		statusStr := "UNKNOWN"
		switch st {
		case statusSuccess:
			statusStr = "SUCCESS"
		case statusFailed:
			statusStr = "FAILED"
		case statusSkipped:
			statusStr = "SKIPPED"
		case statusPending:
			statusStr = "PENDING"
		case statusRunning:
			statusStr = "RUNNING"
		case statusCached:
			statusStr = "CACHED"
		}
		fmt.Printf("  %s: %s\n", step.Name, statusStr)
	}
}

// executeStep runs a single step command.
func executeStep(ctx context.Context, step workflow.Step, opts *Options, env []string) error {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", step.Run)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", step.Run)
	}

	cmd.Env = env

	if opts != nil && opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if opts != nil && opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timeout exceeded")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("exit_code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("execution error: %w", err)
	}

	return nil
}

// Helper: mergeEnv merges varied env sources into a []string "KEY=VALUE" slice
// Base is []string (os.Environ), overrides are map[string]string
func mergeEnv(base []string, overrides ...map[string]string) []string {
	envMap := make(map[string]string)
	
	// Load base
	for _, e := range base {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Apply overrides
	for _, override := range overrides {
		for k, v := range override {
			envMap[k] = v
		}
	}

	// Convert back to slice
	var result []string
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// Helper: validateRequiredEnv checks if keys exist in the environment
func validateRequiredEnv(required []string, env []string) error {
	if len(required) == 0 {
		return nil
	}

	envMap := make(map[string]bool)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) > 0 {
			envMap[parts[0]] = true
		}
	}

	var missing []string
	for _, req := range required {
		if !envMap[req] {
			missing = append(missing, req)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
