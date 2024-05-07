module pocket_rpc

go 1.22.2

require (
	github.com/alitto/pond v1.8.3
	github.com/pokt-foundation/pocket-go v0.17.0
	github.com/puzpuzpuz/xsync v1.5.2
	github.com/rs/zerolog v1.32.0
	github.com/stretchr/testify v1.9.0
	golang.org/x/time v0.5.0
	packages/pocket_rpc v0.0.0-00010101000000-000000000000
	packages/utils v0.0.0-00010101000000-000000000000
)

replace packages/utils => ./../utils
// made this replacement here because when apps/go imports a nested package like common or types will fail
replace packages/pocket_rpc => ./

replace packages/pocket_rpc => ./

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pokt-foundation/utils-go v0.7.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
	golang.org/x/sys v0.12.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
