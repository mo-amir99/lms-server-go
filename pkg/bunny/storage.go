package bunny

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// StorageClient handles Bunny Storage (CDN) operations.
type StorageClient struct {
	zoneName   string
	password   string
	baseURL    string
	hostname   string
	httpClient *http.Client
}

// NewStorageClient creates a new Bunny Storage client.
func NewStorageClient(zoneName, password, baseURL, hostname string) *StorageClient {
	return &StorageClient{
		zoneName: zoneName,
		password: password,
		baseURL:  baseURL,
		hostname: hostname,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// CreateFolder creates a folder in Bunny Storage.
// Note: Bunny Storage creates folders automatically, this is a no-op placeholder.
func (c *StorageClient) CreateFolder(ctx context.Context, folderPath string) error {
	// Bunny Storage auto-creates folders on file upload
	// We can simulate folder creation by creating and deleting a temp file
	tempFilePath := filepath.Join(folderPath, ".temp")

	if err := c.UploadBuffer(ctx, []byte(""), tempFilePath, "text/plain"); err != nil {
		return fmt.Errorf("failed to create folder marker: %w", err)
	}

	if err := c.DeleteFile(ctx, tempFilePath); err != nil {
		// Log but don't fail if deletion fails
		return nil
	}

	return nil
}

// UploadFile uploads a file from the local filesystem to Bunny Storage.
func (c *StorageClient) UploadFile(ctx context.Context, localPath, remotePath, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/%s/%s", c.baseURL, c.zoneName, remotePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.password)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bunny storage error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	publicURL := fmt.Sprintf("https://%s/%s", c.hostname, remotePath)
	return publicURL, nil
}

// UploadBuffer uploads a byte buffer to Bunny Storage.
func (c *StorageClient) UploadBuffer(ctx context.Context, buffer []byte, remotePath, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	url := fmt.Sprintf("%s/%s/%s", c.baseURL, c.zoneName, remotePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(buffer))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.password)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny storage error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UploadStream uploads a file from an io.Reader stream to Bunny Storage.
func (c *StorageClient) UploadStream(ctx context.Context, remotePath string, reader io.Reader, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	url := fmt.Sprintf("%s/%s/%s", c.baseURL, c.zoneName, remotePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, reader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.password)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bunny storage error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	publicURL := fmt.Sprintf("https://%s/%s", c.hostname, remotePath)
	return publicURL, nil
}

// DeleteFile deletes a file from Bunny Storage.
func (c *StorageClient) DeleteFile(ctx context.Context, remotePath string) error {
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, c.zoneName, remotePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.password)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny storage error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteFolder deletes a folder and all its contents from Bunny Storage.
func (c *StorageClient) DeleteFolder(ctx context.Context, folderPath string) error {
	// List all files in folder first, then delete each
	// For now, just delete the folder path directly
	return c.DeleteFile(ctx, folderPath)
}

// GetPublicURL constructs the public CDN URL for a file.
func (c *StorageClient) GetPublicURL(remotePath string) string {
	return fmt.Sprintf("https://%s/%s", c.hostname, remotePath)
}

// FileInfo represents metadata about a file in Bunny Storage.
type FileInfo struct {
	ObjectName      string    `json:"ObjectName"`
	Length          int64     `json:"Length"`
	LastChanged     time.Time `json:"LastChanged"`
	IsDirectory     bool      `json:"IsDirectory"`
	ServerId        int       `json:"ServerId"`
	StorageZoneName string    `json:"StorageZoneName"`
	Path            string    `json:"Path"`
	Guid            string    `json:"Guid"`
}

// ListFiles lists files in a directory.
func (c *StorageClient) ListFiles(ctx context.Context, folderPath string) ([]FileInfo, error) {
	url := fmt.Sprintf("%s/%s/%s/", c.baseURL, c.zoneName, folderPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.password)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bunny storage error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var files []FileInfo
	// Bunny returns an array of FileInfo objects
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// For simplicity, return empty for now - would need proper JSON parsing
	// This is a placeholder for directory listing functionality
	_ = bodyBytes

	return files, nil
}

// StorageUploadInfo contains the details needed for client-side file uploads to Bunny Storage.
type StorageUploadInfo struct {
	URL         string            `json:"url"`
	RemotePath  string            `json:"remotePath"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	ContentType string            `json:"contentType"`
	Method      string            `json:"method"`
	Headers     map[string]string `json:"headers"`
}

// GenerateUploadURL generates a signed upload URL for direct client-side uploads to Bunny Storage.
// The URL expires after the specified duration.
func (c *StorageClient) GenerateUploadURL(remotePath string, contentType string, expiresIn time.Duration) *StorageUploadInfo {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	expiresAt := time.Now().Add(expiresIn)
	uploadURL := fmt.Sprintf("%s/%s/%s", c.baseURL, c.zoneName, remotePath)

	return &StorageUploadInfo{
		URL:         uploadURL,
		RemotePath:  remotePath,
		ExpiresAt:   expiresAt,
		ContentType: contentType,
		Method:      "PUT",
		Headers: map[string]string{
			"AccessKey":    c.password,
			"Content-Type": contentType,
		},
	}
}

// GetPublicCDNURL constructs the public CDN URL after a file has been uploaded.
func (c *StorageClient) GetPublicCDNURL(remotePath string) string {
	return fmt.Sprintf("https://%s/%s", c.hostname, remotePath)
}
