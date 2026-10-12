# Analysis tools

This separate module pins staticcheck and its analysis library without adding
tool dependencies to either application module. The `golang.org/x/tools` override
is needed to read Go 1.27 compiler export data; staticcheck 0.8.1 pins an older
importer.

Run `make check` at the repository root to build the pinned staticcheck binary
and run vet and staticcheck for both applications. `make staticcheck` only builds
the tool into `build/tools/staticcheck`. CI builds the same tool module.
