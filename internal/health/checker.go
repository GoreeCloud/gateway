package health

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/GoreeCloud/goreecloud-gateway/internal/config"
)

type Result struct { BackendID string; Healthy bool; StatusCode int; Err error }

type Checker struct { Client *http.Client }

func New(timeout time.Duration) *Checker { return &Checker{Client:&http.Client{Timeout:timeout}} }

func (c *Checker) Check(ctx context.Context, backend config.Backend) Result {
	if c == nil || c.Client == nil {
		return Result{BackendID: backend.ID, Err: fmt.Errorf("health-check client is unavailable")}
	}
	base, err := url.Parse(backend.URL)
	if err != nil || base.Hostname() == "" || base.User != nil ||
		(base.Scheme != "http" && base.Scheme != "https") ||
		base.Fragment != "" || base.RawQuery != "" || base.ForceQuery {
		return Result{BackendID: backend.ID, Err: fmt.Errorf("invalid backend health-check URL")}
	}
	path := backend.HealthPath
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "?#") {
		return Result{BackendID: backend.ID, Err: fmt.Errorf("invalid backend health-check path")}
	}
	base.Path = path
	base.RawPath = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return Result{BackendID: backend.ID, Err: err}
	}
	// A backend redirect must never cause an implicit probe to a different
	// destination or convert a 3xx into a healthy result. Clone the caller's
	// client rather than changing shared redirect or cookie policy.
	client := *c.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Jar = nil
	resp, err := client.Do(req)
	if err != nil {
		return Result{BackendID: backend.ID, Err: err}
	}
	defer resp.Body.Close()
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	var resultErr error
	if !healthy {
		resultErr = fmt.Errorf("health status %d", resp.StatusCode)
	}
	return Result{BackendID: backend.ID, Healthy: healthy, StatusCode: resp.StatusCode, Err: resultErr}
}
