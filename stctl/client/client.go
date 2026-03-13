package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TransferStats struct {
	Bytes    int64
	Duration time.Duration
}

func (s TransferStats) Speed() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.Bytes) / s.Duration.Seconds()
}

func (s TransferStats) String() string {
	size := formatSize(s.Bytes)
	speed := formatSize(int64(s.Speed()))
	return fmt.Sprintf("%s in %s (%s/s)", size, s.Duration.Round(time.Millisecond), speed)
}

func formatSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: http.DefaultClient,
	}
}

func NewWithTLS(baseURL, certFile, keyFile, caFile string) (*Client, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("parse CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}, nil
}

func cleanPath(remotePath string) string {
	return strings.TrimLeft(remotePath, "/")
}

func (c *Client) PutFile(remotePath, localPath string) (*TransferStats, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	start := time.Now()
	err = c.uploadMultipart(
		http.MethodPut,
		fmt.Sprintf("%s/files/%s", c.BaseURL, cleanPath(remotePath)),
		filepath.Base(localPath),
		f,
	)
	if err != nil {
		return nil, err
	}
	return &TransferStats{Bytes: fi.Size(), Duration: time.Since(start)}, nil
}

func (c *Client) GetFile(remotePath, localPath string) (*TransferStats, error) {
	f, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	return c.GetFileToWriter(remotePath, f)
}

func (c *Client) GetFileToWriter(remotePath string, w io.Writer) (*TransferStats, error) {
	start := time.Now()
	resp, err := c.HTTPClient.Get(fmt.Sprintf("%s/files/%s", c.BaseURL, cleanPath(remotePath)))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return nil, err
	}
	return &TransferStats{Bytes: n, Duration: time.Since(start)}, nil
}

func (c *Client) DeleteFile(remotePath string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/files/%s", c.BaseURL, cleanPath(remotePath)), nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return c.parseError(resp)
	}
	return nil
}

func (c *Client) CreateDir(remotePath string) error {
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/dirs/%s", c.BaseURL, cleanPath(remotePath)), nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

func (c *Client) ListDir(remotePath string) ([]string, error) {
	prefix := cleanPath(remotePath)
	resp, err := c.HTTPClient.Get(fmt.Sprintf("%s/dirs/%s", c.BaseURL, prefix))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var allFiles []string
	if err := json.NewDecoder(resp.Body).Decode(&allFiles); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Ensure prefix ends with "/" for stripping (unless root)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	seen := make(map[string]struct{})
	var children []string
	for _, f := range allFiles {
		rel := strings.TrimPrefix(f, prefix)
		if rel == "" {
			continue
		}
		// Take only the first path segment
		if idx := strings.Index(rel, "/"); idx >= 0 {
			rel = rel[:idx+1] // keep trailing slash to indicate directory
		}
		if _, ok := seen[rel]; !ok {
			seen[rel] = struct{}{}
			children = append(children, rel)
		}
	}

	sort.Strings(children)
	return children, nil
}

func (c *Client) DeleteDir(remotePath string) error {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/dirs/%s", c.BaseURL, cleanPath(remotePath)), nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

func (c *Client) ExtractArchive(remotePath, archivePath, archiveType string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	url := fmt.Sprintf("%s/dirs/%s?extract=%s", c.BaseURL, cleanPath(remotePath), archiveType)
	return c.uploadMultipart(http.MethodPut, url, filepath.Base(archivePath), f)
}

func (c *Client) uploadMultipart(method, url, filename string, r io.Reader) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(part, r); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.CloseWithError(writer.Close())
	}()

	req, err := http.NewRequest(method, url, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
}
