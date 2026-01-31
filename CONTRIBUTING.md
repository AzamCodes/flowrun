# Contributing to Flowrun

Thank you for your interest in contributing to Flowrun!

## Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/flowrun.git
   cd flowrun
   ```

2. **Build the CLI**
   ```bash
   go build -o flowrun main.go
   ```

3. **Run Tests**
   ```bash
   go test ./...
   ```

## Development Guidelines

- **Code Style**: Follow standard Go conventions (`go fmt`).
- **Commits**: Write clear, concise commit messages.
- **Workflow Changes**: If modifying the workflow schema, update `examples/` to reflect changes.

## Compiling for Distribution

We use [GoReleaser](https://goreleaser.com/) for builds.

```bash
goreleaser release --snapshot --clean
```

## Adding New Features

1. Create a new branch.
2. Implement your feature.
3. Add tests (if applicable).
4. Submit a Pull Request.

Happy Coding!
