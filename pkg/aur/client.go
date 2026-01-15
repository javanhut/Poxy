// Package aur provides native AUR (Arch User Repository) support.
package aur

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the default AUR RPC API endpoint
	DefaultBaseURL = "https://aur.archlinux.org/rpc/v5"

	// DefaultTimeout is the default HTTP timeout
	DefaultTimeout = 30 * time.Second
)

// Client is an AUR RPC API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Package represents an AUR package from the RPC API.
type Package struct {
	ID             int     `json:"ID"`
	Name           string  `json:"Name"`
	PackageBaseID  int     `json:"PackageBaseID"`
	PackageBase    string  `json:"PackageBase"`
	Version        string  `json:"Version"`
	Description    string  `json:"Description"`
	URL            string  `json:"URL"`
	NumVotes       int     `json:"NumVotes"`
	Popularity     float64 `json:"Popularity"`
	OutOfDate      *int64  `json:"OutOfDate"` // Unix timestamp, nil if not out of date
	Maintainer     string  `json:"Maintainer"`
	Submitter      string  `json:"Submitter"`
	FirstSubmitted int64   `json:"FirstSubmitted"`
	LastModified   int64   `json:"LastModified"`
	URLPath        string  `json:"URLPath"` // Path to snapshot tarball

	// Dependencies
	Depends      []string `json:"Depends"`
	MakeDepends  []string `json:"MakeDepends"`
	OptDepends   []string `json:"OptDepends"`
	CheckDepends []string `json:"CheckDepends"`
	Conflicts    []string `json:"Conflicts"`
	Provides     []string `json:"Provides"`
	Replaces     []string `json:"Replaces"`

	// Groups and licenses
	Groups   []string `json:"Groups"`
	License  []string `json:"License"`
	Keywords []string `json:"Keywords"`
}

// Response is the AUR RPC API response structure.
type Response struct {
	Version     int       `json:"version"`
	Type        string    `json:"type"`
	ResultCount int       `json:"resultcount"`
	Results     []Package `json:"results"`
	Error       string    `json:"error,omitempty"`
}

// NewClient creates a new AUR client with default settings.
func NewClient() *Client {
	return &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// NewClientWithOptions creates a new AUR client with custom settings.
func NewClientWithOptions(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Search searches for packages matching the query.
func (c *Client) Search(ctx context.Context, query string) ([]Package, error) {
	return c.searchBy(ctx, "search", query)
}

// SearchByName searches for packages by name only.
func (c *Client) SearchByName(ctx context.Context, query string) ([]Package, error) {
	return c.searchBy(ctx, "search", query)
}

// SearchByNameDesc searches for packages by name and description.
func (c *Client) SearchByNameDesc(ctx context.Context, query string) ([]Package, error) {
	return c.searchBy(ctx, "search", query)
}

// SearchByMaintainer searches for packages by maintainer.
func (c *Client) SearchByMaintainer(ctx context.Context, maintainer string) ([]Package, error) {
	return c.searchBy(ctx, "search", maintainer+"&by=maintainer")
}

func (c *Client) searchBy(ctx context.Context, searchType, query string) ([]Package, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, searchType, url.PathEscape(query))

	resp, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// Info retrieves detailed information about one or more packages.
func (c *Client) Info(ctx context.Context, names ...string) ([]Package, error) {
	if len(names) == 0 {
		return nil, nil
	}

	// Build query string with multiple arg[] parameters
	params := make([]string, len(names))
	for i, name := range names {
		params[i] = "arg[]=" + url.QueryEscape(name)
	}

	endpoint := fmt.Sprintf("%s/info?%s", c.baseURL, strings.Join(params, "&"))

	resp, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// GetPackage retrieves detailed information about a single package.
func (c *Client) GetPackage(ctx context.Context, name string) (*Package, error) {
	packages, err := c.Info(ctx, name)
	if err != nil {
		return nil, err
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	return &packages[0], nil
}

// doRequest performs an HTTP GET request to the AUR API.
func (c *Client) doRequest(ctx context.Context, endpoint string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "poxy/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AUR API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var aurResp Response
	if err := json.Unmarshal(body, &aurResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if aurResp.Error != "" {
		return nil, fmt.Errorf("AUR API error: %s", aurResp.Error)
	}

	return &aurResp, nil
}

// GitCloneURL returns the git clone URL for a package.
func (p *Package) GitCloneURL() string {
	return fmt.Sprintf("https://aur.archlinux.org/%s.git", p.PackageBase)
}

// SnapshotURL returns the URL to download the package snapshot tarball.
func (p *Package) SnapshotURL() string {
	return fmt.Sprintf("https://aur.archlinux.org%s", p.URLPath)
}

// IsOutOfDate returns true if the package is marked out of date.
func (p *Package) IsOutOfDate() bool {
	return p.OutOfDate != nil
}

// OutOfDateTime returns the time when the package was marked out of date.
func (p *Package) OutOfDateTime() *time.Time {
	if p.OutOfDate == nil {
		return nil
	}
	t := time.Unix(*p.OutOfDate, 0)
	return &t
}

// FirstSubmittedTime returns the time when the package was first submitted.
func (p *Package) FirstSubmittedTime() time.Time {
	return time.Unix(p.FirstSubmitted, 0)
}

// LastModifiedTime returns the time when the package was last modified.
func (p *Package) LastModifiedTime() time.Time {
	return time.Unix(p.LastModified, 0)
}

// AllDependencies returns all dependencies (depends + makedepends).
func (p *Package) AllDependencies() []string {
	deps := make([]string, 0, len(p.Depends)+len(p.MakeDepends))
	deps = append(deps, p.Depends...)
	deps = append(deps, p.MakeDepends...)
	return deps
}

// IsOrphan returns true if the package has no maintainer.
func (p *Package) IsOrphan() bool {
	return p.Maintainer == ""
}
