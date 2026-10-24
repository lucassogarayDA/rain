// Package httpx ofrece descargas HTTP con reintentos y backoff para los
// repositorios Maven, que ocasionalmente devuelven 429/5xx de forma transitoria.
package httpx

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client envuelve un http.Client con reintentos.
type Client struct {
	HTTP       *http.Client
	MaxRetries int
	BaseDelay  time.Duration
}

// NewClient crea un cliente con valores por defecto razonables.
func NewClient() *Client {
	return &Client{
		HTTP:       &http.Client{Timeout: 10 * time.Minute},
		MaxRetries: 4,
		BaseDelay:  500 * time.Millisecond,
	}
}

// Get realiza una petición GET con reintentos exponenciales ante errores
// transitorios (429, 500, 502, 503, 504) y errores de red. El llamador debe
// cerrar el body de la respuesta.
func (c *Client) Get(url string) (*http.Response, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 10 * time.Minute}
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 4
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = 500 * time.Millisecond
	}

	var lastErr error
	delay := c.BaseDelay
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay *= 2
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "rain/1 (+https://github.com/itskreisler/rain)")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("error de red: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		_ = resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)

		if !retryable(resp.StatusCode) {
			return nil, lastErr
		}
	}
	return nil, lastErr
}

// GetBytes descarga el contenido completo de una URL.
func (c *Client) GetBytes(url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(resp.Body)
}

// retryable indica si un código HTTP merece reintento.
func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
