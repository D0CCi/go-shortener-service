package main

import (
	"log"
	"net/http"

	"github.com/D0CCi/go-shortener-service/internal/httpapi"
	"github.com/D0CCi/go-shortener-service/internal/shortener"
	"github.com/D0CCi/go-shortener-service/internal/storage/memory"
)

func main() {
	svc := shortener.New(memory.New())
	h := httpapi.New(svc, "http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", h))
}
