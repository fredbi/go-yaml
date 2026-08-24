module github.com/go-openapi/go-yaml/internal/benchmarks

go 1.25.0

require (
	github.com/go-openapi/go-yaml v0.0.0-00010101000000-000000000000
	github.com/go-openapi/go-yaml/internal/analysis v0.0.0-00010101000000-000000000000
	go.yaml.in/yaml/v3 v3.0.5
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/go-openapi/go-yaml => ../..

replace github.com/go-openapi/go-yaml/internal/analysis => ../analysis
