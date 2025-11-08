package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Compression levels
const (
	DefaultCompression = gzip.DefaultCompression
	BestSpeed          = gzip.BestSpeed
	BestCompression    = gzip.BestCompression
)

// gzipWriter wraps a gzip.Writer with the ResponseWriter interface
type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func (g *gzipWriter) WriteString(s string) (int, error) {
	return g.writer.Write([]byte(s))
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.writer.Write(data)
}

func (g *gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

// Pool of gzip writers for reuse
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		gz, _ := gzip.NewWriterLevel(io.Discard, DefaultCompression)
		return gz
	},
}

// Compression returns a middleware that compresses responses using gzip.
func Compression(level int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip compression for certain content types
		if !shouldCompress(c.Request) {
			c.Next()
			return
		}

		// Get gzip writer from pool
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		// Reset writer with current response writer
		gz.Reset(c.Writer)
		defer gz.Close()

		// Set compression headers
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// Wrap response writer
		c.Writer = &gzipWriter{
			ResponseWriter: c.Writer,
			writer:         gz,
		}

		c.Next()
	}
}

// shouldCompress determines if the request should be compressed
func shouldCompress(req *http.Request) bool {
	// Check if client accepts gzip
	if !strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	// Don't compress WebSocket connections
	if strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade") {
		return false
	}

	// Don't compress if already compressed (e.g., images, videos)
	contentType := req.Header.Get("Content-Type")
	compressibleTypes := []string{
		"text/",
		"application/json",
		"application/javascript",
		"application/xml",
		"application/x-www-form-urlencoded",
	}

	for _, ct := range compressibleTypes {
		if strings.Contains(contentType, ct) {
			return true
		}
	}

	// Default to compression for empty content type (JSON responses)
	return contentType == ""
}
