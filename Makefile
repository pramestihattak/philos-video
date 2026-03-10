.PHONY: transcode serve build clean

INPUT ?= video.mp4
OUTPUT ?= ./output
PORT ?= 8080

## transcode: encode INPUT into HLS segments under OUTPUT
transcode:
	go run ./cmd/transcode -input $(INPUT) -output $(OUTPUT)

## serve: start HTTP server serving OUTPUT on PORT
serve:
	go run ./cmd/server -dir $(OUTPUT) -port $(PORT)

## build: compile both binaries to ./bin/
build:
	@mkdir -p bin
	go build -o bin/transcode ./cmd/transcode
	go build -o bin/server    ./cmd/server

## clean: remove build artifacts and output directory
clean:
	rm -rf bin/ $(OUTPUT)/

help:
	@grep -E '^##' Makefile | sed 's/## //'
