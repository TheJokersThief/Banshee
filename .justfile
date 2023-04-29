GITHUB_ADDR := "github.com/TheJokersThief/Banshee"
PROJECT_NAME := "banshee"
COMMIT_SHA := `git rev-parse --short HEAD`

# Build for all platforms
build version="development": (build_linux version) (build_darwin version) (build_darwin_arm version)
# Build for linux amd64
build_linux version="development": (_build_generic "linux" "amd64" version)
# Build for MacOS amd64
build_darwin version="development": (_build_generic "darwin" "amd64" version)
# Build for MacOS arm64
build_darwin_arm version="development": (_build_generic "darwin" "arm64" version)

_build_generic os arch version="development":
    GOOS={{ os }} GOARCH={{ arch }} CGO_ENABLED=0 \
        go build \
        --ldflags '-X main.VersionName={{ version }} -X main.GitCommitSHA={{ COMMIT_SHA }}' \
        -o dist/bin/{{ os }}/{{ arch }}/{{ PROJECT_NAME }} ./cmd/{{ PROJECT_NAME }}


# Run code styling and static analysis checks
lint:
    go run github.com/golangci/golangci-lint/cmd/golangci-lint run ./...

@_check_test_conf_exists:
    if `[[ ! -f "examples/global_config/config.test.yaml" ]]`; then \
        echo "\n\nPlease create a test config file by copying examples/global_config/config.yaml to examples/global_config/config.test.yaml\n\n"; exit 1; \
    fi
    if `[[ ! -f "examples/migration_config/migration.test.yaml" ]]`; then \
        echo "\n\nPlease create a test migration file by copying examples/migration_config/migration.yaml to examples/migration_config/migration.test.yaml\n\n"; exit 1; \
    fi

# Run a migration using the test config migration.test.yaml
example_migration: _check_test_conf_exists
    go run cmd/banshee/main.go migrate examples/migration_config/migration.test.yaml \
        --config examples/global_config/config.test.yaml
