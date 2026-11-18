// Command shop serves the quote API on :8080 with a demo catalog.
package main

import (
	"log"
	"net/http"

	"example.com/shop/internal/adapters/inbound/httpapi"
	"example.com/shop/internal/adapters/outbound/memcatalog"
	"example.com/shop/internal/application/checkout"
)

func main() {
	catalog := memcatalog.New(map[string]int64{"apple": 50, "pear": 75})
	h := httpapi.NewHandler(checkout.New(catalog))
	log.Fatal(http.ListenAndServe(":8080", h))
}
