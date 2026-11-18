// Package httpapi is the HTTP inbound adapter: POST /quote.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"example.com/shop/internal/application/checkout"
)

// Quoter is the use case the handler calls.
type Quoter interface {
	Quote(ctx context.Context, items []checkout.Item) (checkout.Quote, error)
}

type itemDTO struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type quoteRequest struct {
	Items []itemDTO `json:"items"`
}

type quoteResponse struct {
	SubtotalCents int64 `json:"subtotal_cents"`
}

// NewHandler returns the API's routes.
func NewHandler(q Quoter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /quote", func(w http.ResponseWriter, r *http.Request) {
		var req quoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Items) == 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		items := make([]checkout.Item, 0, len(req.Items))
		for _, it := range req.Items {
			items = append(items, checkout.Item{SKU: it.SKU, Qty: it.Qty})
		}
		quote, err := q.Quote(r.Context(), items)
		if err != nil {
			http.Error(w, "cannot quote", http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(quoteResponse{SubtotalCents: quote.SubtotalCents})
	})
	return mux
}
