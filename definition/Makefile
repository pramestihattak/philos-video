all: validate

# Serve Swagger UI in the browser at http://localhost:8081
# Requires Docker (already needed for the database).
docs:
	@echo "Swagger UI → http://localhost:8081  (Ctrl+C to stop)"
	docker run --rm -p 8081:8080 \
	  -e SWAGGER_JSON=/api/api.yaml \
	  -v $(CURDIR):/api \
	  swaggerapi/swagger-ui

clean:
	rm -fv api.yaml

SRC_FILES = $(shell find src -name '*.yaml')
build: $(SRC_FILES)
	@echo "Generating api.yaml"
	npx swagger-cli bundle -t yaml src/main.yaml -o api.yaml

validate: clean build
	npx swagger-cli validate api.yaml

generate-gen:
	rm -rf ../gen
	mkdir -p ../gen/api
	oapi-codegen -generate types,chi-server -package api -o ../gen/api/api.gen.go api.yaml