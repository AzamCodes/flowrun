package gui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"flowrun/internal/executor"
	"flowrun/internal/logger"
	"flowrun/internal/workflow"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type App struct {
	fyneApp fyne.App
	window  fyne.Window
	
	// UI Elements
	logEntry    *widget.Entry
	statusLabel *widget.Label
	fileLabel   *widget.Label
	runButton   *widget.Button
	stopButton  *widget.Button
	progressBar *widget.ProgressBar
	
	// State
	currentWorkflow *workflow.Workflow
	cancelFunc      context.CancelFunc
	running         bool
	mutex           sync.Mutex
}

func New() *App {
	a := app.NewWithID("com.flowrun.app")
	w := a.NewWindow("Flowrun")
	w.Resize(fyne.NewSize(800, 600))

	return &App{
		fyneApp: a,
		window:  w,
	}
}

func (a *App) Run() {
	a.setupUI()
	a.window.ShowAndRun()
}

func (a *App) setupUI() {
	// Status Bar
	a.statusLabel = widget.NewLabel("Ready")
	a.fileLabel = widget.NewLabel("No workflow selected")
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Hide()

	// Logs
	a.logEntry = widget.NewMultiLineEntry()
	a.logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	a.logEntry.Wrapping = fyne.TextWrapWord
	a.logEntry.Disable() // Read-only

	// Toolbar
	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), a.openFile)
	a.runButton = widget.NewButtonWithIcon("Run", theme.MediaPlayIcon(), a.runWorkflow)
	a.stopButton = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), a.stopWorkflow)
	a.runButton.Disable()
	a.stopButton.Disable()

	toolbar := container.NewHBox(
		openBtn, 
		layout.NewSpacer(),
		a.runButton, 
		a.stopButton,
	)

	// Layout
	header := container.NewVBox(
		toolbar,
		container.NewHBox(widget.NewLabel("Start File:"), a.fileLabel),
		a.progressBar,
	)
	
	footer := container.NewHBox(a.statusLabel)

	content := container.NewBorder(header, footer, nil, nil, 
		container.NewPadded(a.logEntry),
	)

	a.window.SetContent(content)
}

func (a *App) openFile() {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if reader == nil {
			return // cancelled
		}
		defer reader.Close()

		path := reader.URI().Path()
		// Load workflow to verify it's valid
		wf, err := workflow.Load(path)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Invalid workflow: %w", err), a.window)
			return
		}

		a.mutex.Lock()
		a.currentWorkflow = wf
		a.fileLabel.SetText(path)
		a.runButton.Enable()
		a.statusLabel.SetText("Workflow loaded: " + wf.Name)
		a.logEntry.SetText("") // Clear logs
		a.mutex.Unlock()

	}, a.window)
	
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".yaml", ".yml"}))
	fd.Show()
}

// writerAdapter adapts io.Writer to write to UI
type writerAdapter struct {
	writeFunc func(string)
}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	w.writeFunc(string(p))
	return len(p), nil
}

func (a *App) runWorkflow() {
	a.mutex.Lock()
	if a.running || a.currentWorkflow == nil {
		a.mutex.Unlock()
		return
	}
	a.running = true
	a.runButton.Disable()
	a.stopButton.Enable()
	a.progressBar.Show()
	a.progressBar.SetValue(0)
	a.logEntry.SetText("")
	a.statusLabel.SetText("Running...")
	
	// Setup context
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	wf := a.currentWorkflow
	a.mutex.Unlock()

	// Redirect logger and executor output
	logWriter := &writerAdapter{
		writeFunc: func(s string) {
			// Append to log entry (must be on main thread)
			text := strings.TrimRight(s, "\n")
			if text == "" { return }
			a.window.Canvas().Refresh(a.logEntry)
			a.logEntry.SetText(a.logEntry.Text + text + "\n")
			a.logEntry.CursorRow = len(strings.Split(a.logEntry.Text, "\n"))
		},
	}
	
	// Create a combined writer that writes to UI and (optionally) stdout
	logger.SetOutput(logWriter)
	
	go func() {
		defer func() {
			a.mutex.Lock()
			a.running = false
			a.runButton.Enable()
			a.stopButton.Disable()
			a.progressBar.Hide()
			a.cancelFunc = nil
			a.mutex.Unlock()
		}()

		opts := &executor.Options{
			Stdout: logWriter,
			Stderr: logWriter,
		}

		err := executor.Execute(ctx, wf, opts)
		
		a.mutex.Lock()
		if err != nil {
			// Check if cancelled
			if ctx.Err() == context.Canceled {
				a.statusLabel.SetText("Workflow cancelled")
				logger.Error("Workflow cancelled by user")
			} else {
				a.statusLabel.SetText("Failed")
				logger.Error(err.Error()) // Provide visual error
			}
		} else {
			a.statusLabel.SetText("Success")
			a.progressBar.SetValue(1.0)
		}
		a.mutex.Unlock()
	}()
}

func (a *App) stopWorkflow() {
	a.mutex.Lock()
	if a.cancelFunc != nil {
		a.cancelFunc()
		a.statusLabel.SetText("Stopping...")
	}
	a.mutex.Unlock()
}
