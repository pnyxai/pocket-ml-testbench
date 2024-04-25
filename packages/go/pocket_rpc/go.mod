module pocket_rpc

go 1.21.6

require (
	github.com/alitto/pond v1.8.3
	github.com/pokt-foundation/pocket-go v0.17.0
	github.com/puzpuzpuz/xsync v1.5.2
	github.com/rs/zerolog v1.32.0
	github.com/stretchr/testify v1.8.0
	golang.org/x/time v0.5.0
	packages/utils v0.0.0-00010101000000-000000000000
)

replace packages/utils => ./../utils

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pokt-foundation/utils-go v0.7.0 // indirect
	github.com/stretchr/objx v0.4.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
