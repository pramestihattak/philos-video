.PHONY: bundle validate generate

# Bundle split YAML src files into a single api.yaml (for future use when using split files)
bundle:
	npx --yes @apidevtools/swagger-cli bundle src/main.yaml -o api.yaml -t yaml

# Validate the spec
validate:
	npx --yes @apidevtools/swagger-cli validate api.yaml

# Generate Go types + chi ServerInterface from api.yaml
generate:
	oapi-codegen --config=oapi-codegen.yaml api.yaml
