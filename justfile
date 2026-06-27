# shared — engine-agnostic, tool-agnostic substrate for the ctxloom org.
# Standalone module, consumed by the family via commit-pseudo-version pins.
TOP := `git rev-parse --show-toplevel`

# Show the current version (versionator; reads VERSION + git).
show-version:
    @versionator output version

# Compile-check all packages (library — no binary to stamp).
build:
    go build {{TOP}}/...

# Run the package tests under -race.
test *ARGS:
    go test -race {{ARGS}} {{TOP}}/...

# Vet all packages.
vet:
    go vet {{TOP}}/...

# Tidy module dependencies.
tidy:
    go mod tidy

# CI entrypoint: vet + race tests.
check: vet test
