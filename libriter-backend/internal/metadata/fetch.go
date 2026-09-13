package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// UserAgent identifikuje aplikaci cizím webům. OpenLibrary o slušný
// User-Agent výslovně prosí, scrapery ho mají mít taky.
const UserAgent = "Libriter/1.0 (osobni-audiobook-knihovna; +https://github.com/libriter)"

// Fetcher je HTTP klient s volitelnou prodlevou mezi požadavky. Scrapery
// cizích webů si nastaví MinDelay (slušné chování), oficiální API ji nemají.
type Fetcher struct {
	client   *http.Client
	minDelay time.Duration

	mu      sync.Mutex
	lastReq time.Time
}

func NewFetcher(minDelay time.Duration) *Fetcher {
	return &Fetcher{
		client:   &http.Client{Timeout: 15 * time.Second},
		minDelay: minDelay,
	}
}

// Get stáhne URL. Volající musí tělo zavřít.
func (f *Fetcher) Get(ctx context.Context, targetURL string, accept string) (io.ReadCloser, error) {
	if err := f.wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", accept)
	req.Header.Set("Accept-Language", "cs,en;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &StatusError{Status: resp.StatusCode, URL: targetURL}
	}
	return resp.Body, nil
}

// StatusError je odpověď s jiným stavem než 200. Poskytovatelé podle něj
// rozlišují „nenašlo se“ od „narazili jsme na limit“.
type StatusError struct {
	Status int
	URL    string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("HTTP %d pro %s", e.Status, e.URL)
}

// StatusOf vrátí HTTP status z chyby, nebo 0, pokud chyba nepochází z odpovědi.
func StatusOf(err error) int {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr.Status
	}
	return 0
}

// GetJSON stáhne URL a rozparsuje ji do v.
func (f *Fetcher) GetJSON(ctx context.Context, targetURL string, v any) error {
	body, err := f.Get(ctx, targetURL, "application/json")
	if err != nil {
		return err
	}
	defer body.Close()

	// Strop na velikost odpovědi – cizí server může poslat cokoliv.
	return json.NewDecoder(io.LimitReader(body, 8<<20)).Decode(v)
}

// wait drží minimální odstup mezi požadavky na tentýž zdroj.
func (f *Fetcher) wait(ctx context.Context) error {
	if f.minDelay <= 0 {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if elapsed := time.Since(f.lastReq); elapsed < f.minDelay {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.minDelay - elapsed):
		}
	}
	f.lastReq = time.Now()
	return nil
}
