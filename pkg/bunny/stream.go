package bunny

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// StreamClient handles Bunny Stream API operations.
type StreamClient struct {
	libraryID   string
	apiKey      string
	baseURL     string
	securityKey string
	deliveryURL string
	expiresIn   int
	httpClient  *http.Client
}

// NewStreamClient creates a new Bunny Stream client.
func NewStreamClient(libraryID, apiKey, baseURL, securityKey, deliveryURL string, expiresIn int) *StreamClient {
	return &StreamClient{
		libraryID:   libraryID,
		apiKey:      apiKey,
		baseURL:     baseURL,
		securityKey: securityKey,
		deliveryURL: deliveryURL,
		expiresIn:   expiresIn,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateCollectionRequest represents the payload for creating a collection.
type CreateCollectionRequest struct {
	Name string `json:"name"`
}

// CreateCollectionResponse represents the response from creating a collection.
type CreateCollectionResponse struct {
	GUID string `json:"guid"`
}

// CreateCourseCollection creates a new collection for a course.
func (c *StreamClient) CreateCourseCollection(ctx context.Context, subscriptionIdentifierName, courseName string) (string, error) {
	collectionName := fmt.Sprintf("%s - %s", subscriptionIdentifierName, courseName)

	reqBody := CreateCollectionRequest{
		Name: collectionName,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/library/%s/collections", c.baseURL, c.libraryID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var result CreateCollectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.GUID, nil
}

// DeleteCollection deletes a collection by ID.
func (c *StreamClient) DeleteCollection(ctx context.Context, collectionID string) error {
	url := fmt.Sprintf("%s/library/%s/collections/%s", c.baseURL, c.libraryID, collectionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UpdateCollectionRequest represents the payload for updating a collection name.
type UpdateCollectionRequest struct {
	Name string `json:"name"`
}

// UpdateCollection updates a collection's name with proper formatting (subscriptionIdentifier - courseName).
func (c *StreamClient) UpdateCollection(ctx context.Context, collectionID, subscriptionIdentifierName, courseName string) error {
	if collectionID == "" || subscriptionIdentifierName == "" || courseName == "" {
		return fmt.Errorf("collectionID, subscriptionIdentifierName, and courseName are required")
	}

	// Format collection name to match creation style: "subscription - courseName"
	collectionName := fmt.Sprintf("%s - %s", subscriptionIdentifierName, courseName)

	reqBody := UpdateCollectionRequest{
		Name: collectionName,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/library/%s/collections/%s", c.baseURL, c.libraryID, collectionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateVideoRequest represents the payload for creating a video.
type CreateVideoRequest struct {
	Title        string `json:"title"`
	CollectionID string `json:"collectionId"`
}

// CreateVideoResponse represents the response from creating a video.
type CreateVideoResponse struct {
	GUID string `json:"guid"`
}

// UploadVideoResult contains the result of a video upload.
type UploadVideoResult struct {
	BunnyVideoID string
	VideoURL     string
}

// CreateVideo creates a new video entry in Bunny Stream.
func (c *StreamClient) CreateVideo(ctx context.Context, title, collectionID string) (string, error) {
	reqBody := CreateVideoRequest{
		Title:        title,
		CollectionID: collectionID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/library/%s/videos", c.baseURL, c.libraryID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var result CreateVideoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.GUID, nil
}

// UploadVideoFile uploads a video file to Bunny Stream.
func (c *StreamClient) UploadVideoFile(ctx context.Context, videoID, filePath string, resolutions string) error {
	if resolutions == "" {
		resolutions = "360p,720p"
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/library/%s/videos/%s?enabledResolutions=%s", c.baseURL, c.libraryID, videoID, resolutions)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, file)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteVideo deletes a video by ID.
func (c *StreamClient) DeleteVideo(ctx context.Context, videoID string) error {
	url := fmt.Sprintf("%s/library/%s/videos/%s", c.baseURL, c.libraryID, videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetVideoStatus retrieves the processing status of a video.
type VideoStatus struct {
	GUID           string  `json:"guid"`
	Title          string  `json:"title"`
	Status         int     `json:"status"` // 0=queued, 1=processing, 2=encoding, 3=finished, 4=resolution_finished, 5=failed
	AvgWatchTime   float64 `json:"averageWatchTime"`
	TotalWatchTime float64 `json:"totalWatchTime"`
	Views          int     `json:"views"`
}

func (c *StreamClient) GetVideoStatus(ctx context.Context, videoID string) (*VideoStatus, error) {
	url := fmt.Sprintf("%s/library/%s/videos/%s", c.baseURL, c.libraryID, videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("AccessKey", c.apiKey)
	req.Header.Set("User-Agent", "LMS-Server-Go/1.0.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bunny API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	var status VideoStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// SignedVideoURL generates a signed Bunny Stream playlist URL matching the legacy Node implementation.
func (c *StreamClient) SignedVideoURL(videoID string) (string, error) {
	if strings.TrimSpace(videoID) == "" {
		return "", fmt.Errorf("videoID is required")
	}
	if strings.TrimSpace(c.securityKey) == "" || strings.TrimSpace(c.deliveryURL) == "" {
		return "", fmt.Errorf("bunny stream signing configuration is missing")
	}

	delivery := strings.TrimSpace(c.deliveryURL)
	if !strings.HasPrefix(delivery, "http://") && !strings.HasPrefix(delivery, "https://") {
		delivery = "https://" + delivery
	}

	if !strings.HasSuffix(delivery, "/") {
		delivery += "/"
	}

	expiresIn := c.expiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	expiration := time.Now().Unix() + int64(expiresIn)
	path := fmt.Sprintf("%s/playlist.m3u8", strings.Trim(strings.TrimPrefix(videoID, "/"), "/"))
	urlPath := "/" + path

	stringToSign := fmt.Sprintf("%s%s%d", c.securityKey, urlPath, expiration)
	hash := sha256.Sum256([]byte(stringToSign))
	token := base64.StdEncoding.EncodeToString(hash[:])
	token = strings.NewReplacer("+", "-", "/", "_", "=", "").Replace(token)

	return fmt.Sprintf("%s%s?token=%s&expires=%d", strings.TrimRight(delivery, "/"), urlPath, token, expiration), nil
}

// CreateVideoUploadURL creates a video entry and returns a signed upload URL for direct client upload
func (c *StreamClient) CreateVideoUploadURL(ctx context.Context, title, collectionID string, expirationSeconds int) (string, string, error) {
	// Create video entry first
	videoID, err := c.CreateVideo(ctx, title, collectionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to create video: %w", err)
	}

	// Generate upload URL (client will use API key in Authorization header)
	uploadURL := fmt.Sprintf("%s/library/%s/videos/%s", c.baseURL, c.libraryID, videoID)

	// Return video ID and upload URL
	return videoID, uploadURL, nil
}

// TusUploadInfo returns information needed for TUS resumable upload to Bunny Stream
type TusUploadInfo struct {
	VideoID                string `json:"videoId"`
	LessonName             string `json:"lessonName"`  // The lesson name for client reference
	TusEndpoint            string `json:"tusEndpoint"` // TUS upload endpoint
	LibraryID              string `json:"libraryId"`
	AuthorizationSignature string `json:"authorizationSignature"` // Signed auth token
	AuthorizationExpire    int64  `json:"authorizationExpire"`    // Unix timestamp when signature expires
	ExpiresInSec           int    `json:"expiresIn"`              // Seconds until expiration
}

// GenerateTusUploadInfo creates a video and returns TUS upload information with signed authentication
// TUS Protocol enables resumable uploads - if connection fails, upload can resume from where it left off
// Reference: https://docs.bunny.net/docs/stream-upload-videos#upload-with-tus
func (c *StreamClient) GenerateTusUploadInfo(ctx context.Context, title, collectionID string, expirationSeconds int) (*TusUploadInfo, error) {
	// Create video entry in Bunny Stream
	videoID, err := c.CreateVideo(ctx, title, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create video: %w", err)
	}

	if expirationSeconds <= 0 {
		expirationSeconds = 21600 // Default 6 hours for large video uploads
	}

	expiration := time.Now().Unix() + int64(expirationSeconds)

	// Generate signature for TUS authentication
	// Format: SHA256(libraryId + apiKey + expirationTime + videoId)
	signatureString := fmt.Sprintf("%s%s%d%s", c.libraryID, c.apiKey, expiration, videoID)
	hash := sha256.Sum256([]byte(signatureString))
	signature := fmt.Sprintf("%x", hash)

	// TUS endpoint for resumable uploads
	tusEndpoint := "https://video.bunnycdn.com/tusupload"

	return &TusUploadInfo{
		VideoID:                videoID,
		LessonName:             title,
		TusEndpoint:            tusEndpoint,
		LibraryID:              c.libraryID,
		AuthorizationSignature: signature,
		AuthorizationExpire:    expiration,
		ExpiresInSec:           expirationSeconds,
	}, nil
}
