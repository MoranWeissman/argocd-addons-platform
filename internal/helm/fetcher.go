package helm

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChartVersion represents a version entry from a Helm repo index.
type ChartVersion struct {
	Version    string   `yaml:"version"`
	URLs       []string `yaml:"urls"`
	AppVersion string   `yaml:"appVersion,omitempty"`
}

// repoIndex represents a Helm repository index.yaml.
type repoIndex struct {
	Entries map[string][]ChartVersion `yaml:"entries"`
}

// Fetcher downloads Helm chart values.yaml for comparison.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a new Helm chart fetcher.
func NewFetcher() *Fetcher {
	return &Fetcher{client: &http.Client{}}
}

// ListVersions returns available versions for a chart from the repo index.
func (f *Fetcher) ListVersions(ctx context.Context, repoURL, chartName string) ([]ChartVersion, error) {
	// Fetch index.yaml from the Helm repo
	indexURL := strings.TrimRight(repoURL, "/") + "/index.yaml"
	req, err := http.NewRequestWithContext(ctx, "GET", indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching index returned status %d", resp.StatusCode)
	}

	// Limit to 10MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading index: %w", err)
	}

	var idx repoIndex
	if err := yaml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	versions, ok := idx.Entries[chartName]
	if !ok {
		return nil, fmt.Errorf("chart %q not found in repo", chartName)
	}

	return versions, nil
}

// FetchValues downloads a chart archive and extracts values.yaml.
func (f *Fetcher) FetchValues(ctx context.Context, repoURL, chartName, version string) (string, error) {
	// First get the chart URL from index
	versions, err := f.ListVersions(ctx, repoURL, chartName)
	if err != nil {
		return "", err
	}

	var chartURL string
	for _, v := range versions {
		if v.Version == version && len(v.URLs) > 0 {
			chartURL = v.URLs[0]
			break
		}
	}
	if chartURL == "" {
		return "", fmt.Errorf("version %s not found for chart %s", version, chartName)
	}

	// Handle relative URLs
	if !strings.HasPrefix(chartURL, "http") {
		chartURL = strings.TrimRight(repoURL, "/") + "/" + chartURL
	}

	// Download the .tgz
	req, err := http.NewRequestWithContext(ctx, "GET", chartURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating chart request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading chart: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chart download returned %d", resp.StatusCode)
	}

	// Extract values.yaml from tar.gz
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("decompressing: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading tar: %w", err)
		}

		// values.yaml is typically at {chartName}/values.yaml
		if strings.HasSuffix(header.Name, "/values.yaml") || header.Name == "values.yaml" {
			data, err := io.ReadAll(io.LimitReader(tr, 5*1024*1024))
			if err != nil {
				return "", fmt.Errorf("reading values.yaml: %w", err)
			}
			return string(data), nil
		}
	}

	return "", fmt.Errorf("values.yaml not found in chart archive")
}
