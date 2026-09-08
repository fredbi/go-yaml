module github.com/go-openapi/go-yaml/internal/benchmarks

go 1.25.0

require (
	github.com/go-openapi/go-yaml v0.0.0-00010101000000-000000000000
	github.com/go-openapi/go-yaml/internal/analysis v0.0.0-00010101000000-000000000000
	github.com/go-openapi/spec v0.22.0
	github.com/go-openapi/testify/v2 v2.6.1
	go.yaml.in/yaml/v3 v3.0.5
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.2 // indirect
	github.com/go-openapi/swag/conv v0.25.1 // indirect
	github.com/go-openapi/swag/jsonname v0.25.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.1 // indirect
	github.com/go-openapi/swag/loading v0.25.1 // indirect
	github.com/go-openapi/swag/stringutils v0.25.1 // indirect
	github.com/go-openapi/swag/typeutils v0.25.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.1 // indirect
)

replace github.com/go-openapi/go-yaml => ../..

replace github.com/go-openapi/go-yaml/internal/analysis => ../analysis
