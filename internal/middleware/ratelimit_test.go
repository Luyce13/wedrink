package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wedrink/internal/middleware"
)

func TestRateLimitMiddleware(t *testing.T) {
	// Limiter with capacity 2 and refill rate 1/sec
	limiter := middleware.Limit(2, 1.0)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handlerToTest := limiter(dummyHandler)

	// First request: OK (tokens = 1)
	req1 := httptest.NewRequest("POST", "/login", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	rr1 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on first request, got %d", rr1.Code)
	}

	// Second request: OK (tokens = 0)
	req2 := httptest.NewRequest("POST", "/login", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	rr2 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on second request, got %d", rr2.Code)
	}

	// Third request: 429 Too Many Requests (tokens empty)
	req3 := httptest.NewRequest("POST", "/login", nil)
	req3.RemoteAddr = "10.0.0.1:1234"
	rr3 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected HTTP 429 on third request, got %d", rr3.Code)
	}

	// Wait 1.1s for token refill
	time.Sleep(1100 * time.Millisecond)

	// Fourth request: OK after refill
	req4 := httptest.NewRequest("POST", "/login", nil)
	req4.RemoteAddr = "10.0.0.1:1234"
	rr4 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 after refill, got %d", rr4.Code)
	}
}
