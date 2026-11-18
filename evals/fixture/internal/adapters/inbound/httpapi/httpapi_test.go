package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/shop/internal/application/checkout"
)

type stubQuoter struct {
	quote checkout.Quote
	err   error
}

func (s stubQuoter) Quote(context.Context, []checkout.Item) (checkout.Quote, error) {
	return s.quote, s.err
}

func post(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/quote", strings.NewReader(body)))
	return rec
}

func TestQuote_ReturnsTheSubtotal(t *testing.T) {
	rec := post(t, NewHandler(stubQuoter{quote: checkout.Quote{SubtotalCents: 399}}), `{"items":[{"sku":"a","qty":1}]}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"subtotal_cents":399`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuote_RejectsAnEmptyOrder(t *testing.T) {
	if rec := post(t, NewHandler(stubQuoter{}), `{"items":[]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestQuote_MapsUseCaseErrorsTo422(t *testing.T) {
	if rec := post(t, NewHandler(stubQuoter{err: errors.New("x")}), `{"items":[{"sku":"a","qty":1}]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code = %d, want 422", rec.Code)
	}
}
