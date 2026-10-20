# Analysis tools

This separate module pins staticcheck and its analysis library without adding
tool dependencies to either application module. The `golang.org/x/tools`
dependency is pinned to the latest release for Go 1.27 compiler export-data
support, alongside staticcheck 0.8.2.

Run `make check` at the repository root to build the pinned staticcheck binary
and run vet and staticcheck for both applications. `make staticcheck` only builds
the tool into `build/tools/staticcheck`. CI builds the same tool module.
