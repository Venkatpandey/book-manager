package main

import (
	"book-manager/api"
	"book-manager/internal/adapter"
	"book-manager/internal/core"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "Listen address")
	logLevel := flag.String("log-level", "info", "Log level")
	flag.Parse()

	router := chi.NewRouter()
	lvl := new(slog.LevelVar)
	err := lvl.UnmarshalText([]byte(*logLevel))
	if err != nil {
		lvl.Set(slog.LevelInfo)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	}))

	bookRepo := adapter.NewBookRepo()
	service := core.NewService(bookRepo, nil)
	httpHandler := adapter.NewHTTPHandler(service, logger)

	api.HandlerFromMux(httpHandler, router)

	log.Printf("listening on %s", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, router); err != nil {
		log.Fatal(err)
	}
}
