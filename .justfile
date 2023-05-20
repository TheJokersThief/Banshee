GITHUB_ADDR := "github.com/TheJokersThief/Banshee"
PROJECT_NAME := "banshee"
COMMIT_SHA := `git rev-parse --short HEAD`

# Build for all platforms
build_all version="development": (build_linux version) (build_darwin version) (build_darwin_arm version)
# Build for linux amd64
build_linux version="development": (_build_generic "linux" "amd64" version)
# Build for MacOS amd64
build_darwin version="development": (_build_generic "darwin" "amd64" version)
# Build for MacOS arm64
build_darwin_arm version="development": (_build_generic "darwin" "arm64" version)

# Run code styling and static analysis checks
lint:
    go get github.com/golangci/golangci-lint/cmd/golangci-lint
    go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...

# Push a tag to mark a new version
publish:
    @echo "Last Tag: $(git describe --tags --abbrev=0)"
    @echo "Commit: $(git log --oneline -1 `git describe --tags --abbrev=0`)"
    @echo ""
    @read -r -p "What version would you like to publish? " VERSION; \
    git tag -a "${VERSION}" -m "${VERSION}"
    git push --tags

# Run a migration using the test config migration.test.yaml
example_migration: _check_test_conf_exists
    go run cmd/banshee/main.go migrate examples/migration_config/migration.test.yaml \
        --config examples/global_config/config.test.yaml

# Compile a binary for the given OS and architecture, annotating it with a version
_build_generic os arch version="development":
    GOOS={{ os }} GOARCH={{ arch }} CGO_ENABLED=0 \
        go build \
        --ldflags '-X main.Version={{ version }} -X main.GitCommitSHA={{ COMMIT_SHA }}' \
        -o dist/bin/{{ PROJECT_NAME }}-{{ os }}-{{ arch }} ./cmd/{{ PROJECT_NAME }}

# Check if the test files exist, and error if they don't 
@_check_test_conf_exists:
    if `[[ ! -f "examples/global_config/config.test.yaml" ]]`; then \
        echo "\n\nPlease create a test config file by copying examples/global_config/config.yaml to examples/global_config/config.test.yaml\n\n"; exit 1; \
    fi
    if `[[ ! -f "examples/migration_config/migration.test.yaml" ]]`; then \
        echo "\n\nPlease create a test migration file by copying examples/migration_config/migration.yaml to examples/migration_config/migration.test.yaml\n\n"; exit 1; \
    fi
