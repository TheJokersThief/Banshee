# Contributing to Banshee

## Development Environment

### Requirements

- Go 1.20 or later
- Git

### Setup

1. Clone the repository:

```bash
git clone https://github.com/TheJokersThief/Banshee.git
cd Banshee
```

2. Download dependencies:

```bash
go mod download
```

3. Build the CLI locally:

```bash
go build -o banshee ./cmd/banshee
```

4. Verify the build:

```bash
./banshee version
```

## Development Workflow

### Building and Testing

Build the CLI:

```bash
go build -o banshee ./cmd/banshee
```

Run all tests:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

### Code Organization

- `cmd/banshee/` - CLI entry point and command definitions
- `pkg/actions/` - Migration action implementations (replace, add_file, run_command, yaml)
- `pkg/configs/` - Configuration file parsing
- `pkg/core/` - Core migration logic (clone, migrate, list, merge)
- `pkg/github/` - GitHub API interactions
- `pkg/progress/` - Progress tracking and batching

### Adding a New Action

1. Create a new file in `pkg/actions/` (e.g., `my_action.go`)
2. Implement the `ActionRunner` interface with a `Run()` method
3. Add a case to the `RunAction()` function in `pkg/actions/actions.go`
4. Document the action in `docs/migrations.md`
5. Add tests (create a `my_action_test.go` file)

## Target Use Case

Banshee is designed for organizations with these characteristics:

- 1000+ repositories on GitHub
- Extensive use of branch protections
- CODEOWNERS files for automatic reviewer assignment
- Need to perform coordinated code changes across many repos

## Testing Your Changes

1. Create a test migration configuration
2. Use a test organization or test repositories
3. Run with `--help` to verify command structure
4. Test with dry-run or small batches first when possible

## Code Style

Follow Go conventions:
- Use `gofmt` for code formatting
- Use meaningful variable and function names
- Add comments for exported functions
- Write tests for new functionality

## Submitting Changes

When submitting a pull request:
1. Ensure tests pass (`go test ./...`)
2. Update documentation if adding new features
3. Add example configs if introducing new capabilities
4. Keep commit messages clear and descriptive
