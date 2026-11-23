module github.com/mvt-project/androidqf/tools

go 1.27.4

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20261120130654-b6de8d37927a // indirect
	golang.org/x/mod v0.43.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	// Keep the Go 1.27 export-data importer up to date.
	golang.org/x/tools v0.53.0 // indirect
	honnef.co/go/tools v0.8.2 // indirect
)

tool honnef.co/go/tools/cmd/staticcheck
