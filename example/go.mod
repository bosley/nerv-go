module nerv-cli

go 1.22.2

replace github.com/bosley/nerv-go => ../

require (
	github.com/bosley/nerv-go v0.0.0-00010101000000-000000000000
	github.com/bosley/nerv-go/modhttp v0.0.0-00010101000000-000000000000
)

replace github.com/bosley/nerv-go/modhttp => ../modhttp
