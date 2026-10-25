package httpx

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fastClient devuelve un cliente con backoff mínimo para tests.
func fastClient(retries int) *Client {
	return &Client{
		HTTP:       &http.Client{Timeout: 5 * time.Second},
		MaxRetries: retries,
		BaseDelay:  time.Millisecond,
	}
}

func TestGetOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "rain/1 (+https://github.com/itskreisler/rain)" {
			t.Errorf("User-Agent = %q", got)
		}
		_, _ = w.Write([]byte("hola"))
	}))
	defer srv.Close()

	resp, err := fastClient(2).Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}

func TestGetRetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	body, err := fastClient(4).GetBytes(srv.URL)
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Errorf("llamadas = %d, quiero 3 (2 fallos + 1 éxito)", n)
	}
}

func TestGetRetriesExhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	resp, err := fastClient(2).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("se esperaba error tras agotar reintentos")
	}
	// 1 intento inicial + 2 reintentos.
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Errorf("llamadas = %d, quiero 3", n)
	}
}

func TestGetNoRetryOnNotFound(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	resp, err := fastClient(4).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("404 debería devolver error")
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("llamadas = %d, un 404 no debe reintentarse", n)
	}
}

func TestRetryable(t *testing.T) {
	retry := []int{429, 500, 502, 503, 504}
	for _, code := range retry {
		if !retryable(code) {
			t.Errorf("retryable(%d) = false, quiero true", code)
		}
	}
	for _, code := range []int{200, 301, 400, 401, 403, 404} {
		if retryable(code) {
			t.Errorf("retryable(%d) = true, quiero false", code)
		}
	}
}

func TestGetDefaultsApplied(t *testing.T) {
	// Un cliente con ceros debe autocorregirse y funcionar.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := &Client{BaseDelay: time.Millisecond}
	if _, err := c.GetBytes(srv.URL); err != nil {
		t.Fatalf("GetBytes con defaults: %v", err)
	}
	if c.MaxRetries != 4 {
		t.Errorf("MaxRetries = %d, quiero 4", c.MaxRetries)
	}
}
