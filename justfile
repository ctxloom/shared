# shared — engine-agnostic, tool-agnostic substrate for the ctxloom org.
# Standalone module; tests run on the host (no devcontainer, no build tags).
TOP := `git rev-parse --show-toplevel`

# Run the package tests under -race.
test *ARGS:
    go test -race {{ARGS}} {{TOP}}/...

# Vet all packages.
vet:
    go vet {{TOP}}/...

# Tidy module dependencies.
tidy:
    go mod tidy
