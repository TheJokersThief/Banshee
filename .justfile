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
