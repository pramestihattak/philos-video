package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"philos-video/internal/server"
)

func main() {
	dir := flag.String("dir", "./output", "Directory containing HLS output")
	port := flag.Int("port", 8080, "HTTP port to listen on")
	flag.Parse()

	if _, err := os.Stat(*dir); err != nil {
		slog.Error("output directory not found", "dir", *dir, "err", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf(":%d", *port)
	mux := server.NewServer(*dir, *port)

	slog.Info("server starting",
		"addr", fmt.Sprintf("http://localhost%s", addr),
		"dir", *dir,
	)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
