module github.com/mvt-project/androidqf/tools

go 1.27.2

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20261011100332-faecfb24eb49 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	// Go 1.27 export data requires a newer importer than staticcheck pins.
	golang.org/x/tools v0.52.0 // indirect
	honnef.co/go/tools v0.8.1 // indirect
)

tool honnef.co/go/tools/cmd/staticcheck
