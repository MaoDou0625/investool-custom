package yahoo

import (
	"net/http"
	"time"
)

// Yahoo is a small Yahoo Finance client for public chart endpoints.
type Yahoo struct {
	HTTPClient *http.Client
}

func NewYahoo() Yahoo {
	return Yahoo{
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}
