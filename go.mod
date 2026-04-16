module github.com/szuend/tscc

go 1.26

require (
	github.com/microsoft/typescript-go v0.0.0-00010101000000-000000000000
	github.com/rogpeppe/go-internal v1.14.1
	github.com/spf13/pflag v1.0.5
)

require (
	github.com/go-json-experiment/json v0.0.0-20260214004413-d219187c3433 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
)

replace github.com/microsoft/typescript-go => ./third_party/typescript-go
