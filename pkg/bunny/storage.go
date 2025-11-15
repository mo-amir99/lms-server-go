package bunny

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// ExtractRelativePath extracts the relative storage path from a full CDN URL.
// For example, converts "https://elites-academy.b-cdn.net/test-sub/course-id/file.pdf"
// to "test-sub/course-id/file.pdf"
func (c *StorageClient) ExtractRelativePath(cdnURL string) string {
	// Remove the CDN hostname prefix
	prefix := fmt.Sprintf("https://%s/", c.hostname)
	if len(cdnURL) > len(prefix) && cdnURL[:len(prefix)] == prefix {
		return cdnURL[len(prefix):]
	}
	// If it doesn't match the expected format, return as-is (might already be relative)
	return cdnURL
}

// BunnyTime is a custom time type that handles Bunny Storage's timestamp format
type BunnyTime struct {
	time.Time
}

// UnmarshalJSON parses Bunny's timestamp format (without timezone)
func (bt *BunnyTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		bt.Time = time.Time{}
		return nil
	}

	// Bunny returns timestamps like "2025-11-13T14:36:25.178" without timezone
	// We'll try multiple formats
	formats := []string{
		"2006-01-02T15:04:05.999", // With milliseconds
		"2006-01-02T15:04:05",     // Without milliseconds
		time.RFC3339,              // Standard RFC3339
	}

	var err error
	for _, format := range formats {
		bt.Time, err = time.Parse(format, s)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("unable to parse time: %s", s)
}

// FileInfo represents metadata about a file in Bunny Storage.
type FileInfo struct {
	ObjectName      string    `json:"ObjectName"`
	Length          int64     `json:"Length"`
	LastChanged     BunnyTime `json:"LastChanged"`
	IsDirectory     bool      `json:"IsDirectory"`
	ServerId        int       `json:"ServerId"`
	StorageZoneName string    `json:"StorageZoneName"`
	Path            string    `json:"Path"`
	Guid            string    `json:"Guid"`
}

// ListFiles lists files in a directory.
func (c *StorageClient) ListFiles(ctx context.Context, folderPath string) ([]FileInfo, error) {
	url := c.buildFolderURL(folderPath)
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
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode bunny storage listing: %w", err)
	}

	return files, nil
}

// CalculateFolderSize recursively sums the size of every file under folderPath.
func (c *StorageClient) CalculateFolderSize(ctx context.Context, folderPath string) (int64, error) {
	items, err := c.ListFiles(ctx, folderPath)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, item := range items {
		if item.IsDirectory {
			subPath := joinStoragePaths(folderPath, item.ObjectName)
			size, err := c.CalculateFolderSize(ctx, subPath)
			if err != nil {
				return 0, err
			}
			total += size
		} else {
			total += item.Length
		}
	}

	return total, nil
}

func (c *StorageClient) buildFolderURL(folderPath string) string {
	path := strings.Trim(folderPath, "/")
	base := fmt.Sprintf("%s/%s", strings.TrimRight(c.baseURL, "/"), c.zoneName)
	if path != "" {
		base = fmt.Sprintf("%s/%s", base, path)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base
}

func joinStoragePaths(parts ...string) string {
	result := ""
	for _, part := range parts {
		trimmed := strings.Trim(part, "/")
		if trimmed == "" {
			continue
		}
		if result == "" {
			result = trimmed
		} else {
			result = result + "/" + trimmed
		}
	}
	return result
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
// The URL expires after the specified duration and includes authentication signature.
func (c *StorageClient) GenerateUploadURL(remotePath string, contentType string, expiresIn time.Duration) *StorageUploadInfo {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	expiresAt := time.Now().Add(expiresIn)
	expiration := expiresAt.Unix()

	// Generate signature for pre-signed URL
	// Format: SHA256(zoneName + password + expiration + remotePath)
	signatureString := fmt.Sprintf("%s%s%d%s", c.zoneName, c.password, expiration, remotePath)
	hash := sha256.New()
	hash.Write([]byte(signatureString))
	signature := fmt.Sprintf("%x", hash.Sum(nil))

	// Create pre-signed upload URL with signature as query parameter
	uploadURL := fmt.Sprintf("%s/%s/%s?signature=%s&expires=%d",
		c.baseURL, c.zoneName, remotePath, signature, expiration)

	return &StorageUploadInfo{
		URL:         uploadURL,
		RemotePath:  remotePath,
		ExpiresAt:   expiresAt,
		ContentType: contentType,
		Method:      "PUT",
		Headers: map[string]string{
			"Content-Type": contentType,
		},
	}
}

// GetPublicCDNURL constructs the public CDN URL after a file has been uploaded.
func (c *StorageClient) GetPublicCDNURL(remotePath string) string {
	return fmt.Sprintf("https://%s/%s", c.hostname, remotePath)
}
