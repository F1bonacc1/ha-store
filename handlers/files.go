package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/f1bonacc1/ha-store/store"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

const (
	jsRetryAttempts = 5
	jsRetryDelay    = time.Second
)

type FileHandler struct {
	Store          *store.Store
	ThrottleSpeed  int64
	UploadDeadline time.Duration
	DeleteDeadline time.Duration
}

func (h *FileHandler) HandlePutFile(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/") // Remove leading slash

	if path == "" || strings.HasSuffix(path, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
		return
	}

	bucket := h.Store.GetBucket()

	// Read file from multipart form
	file, err := c.FormFile("file")
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to get file from form")
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get file from form"})
		return
	}

	src, err := file.Open()
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to open file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer src.Close()

	// Throttle upload to the configured speed to prevent overwhelming JetStream's
	// internal pending queue on replicated clusters
	throttledReader := NewThrottledReader(src, h.ThrottleSpeed)

	// Read optional metadata form fields
	permissions := c.PostForm("permissions")
	if permissions == "" {
		permissions = "0644"
	}
	owner := c.PostForm("owner")
	if owner == "" {
		owner = ownerFromTLS(c)
	}
	group := c.PostForm("group")

	// Use streaming Put with larger chunk size (1MB) to reduce IPQ pressure
	// when using replicated JetStream clusters
	meta := jetstream.ObjectMeta{
		Name: path,
		Metadata: map[string]string{
			"permissions": permissions,
			"owner":       owner,
			"group":       group,
		},
		Opts: &jetstream.ObjectMetaOptions{
			ChunkSize: 1024 * 1024, // 1MB chunks (default is 128KB)
		},
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.UploadDeadline)
	defer cancel()

	for attempt := range jsRetryAttempts {
		if attempt > 0 {
			// Reopen the file since the previous reader was consumed
			src.Close()
			src, err = file.Open()
			if err != nil {
				log.Error().Err(err).Str("path", path).Msg("failed to reopen file for retry")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reopen file"})
				return
			}
			throttledReader = NewThrottledReader(src, h.ThrottleSpeed)
			log.Warn().Int("attempt", attempt+1).Str("path", path).Msg("retrying upload after stream error")
			time.Sleep(jsRetryDelay)
		}
		_, err = bucket.Put(ctx, meta, throttledReader)
		if err == nil {
			break
		}
		if !errors.Is(err, jetstream.ErrNoStreamResponse) {
			break
		}
	}
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to save file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to save file: %v", err)})
		return
	}

	c.Status(http.StatusOK)
}

func (h *FileHandler) HandleGetFile(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/")

	bucket := h.Store.GetBucket()

	// Add timeout to the request context to prevent hanging
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.UploadDeadline)
	defer cancel()

	var (
		obj jetstream.ObjectResult
		err error
	)
	for attempt := range jsRetryAttempts {
		if attempt > 0 {
			log.Warn().Int("attempt", attempt+1).Str("path", path).Msg("retrying download after stream error")
			time.Sleep(jsRetryDelay)
		}
		obj, err = bucket.Get(ctx, path)
		if err == nil {
			break
		}
		if !errors.Is(err, jetstream.ErrNoStreamResponse) {
			break
		}
	}
	if err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			log.Warn().Str("path", path).Msg("file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		log.Error().Err(err).Str("path", path).Msg("failed to get file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get file: %v", err)})
		return
	}

	info, err := obj.Info()
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("failed to get object info")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get object info"})
		return
	}

	// Extract filename from path
	parts := strings.Split(path, "/")
	filename := parts[len(parts)-1]

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.DataFromReader(http.StatusOK, int64(info.Size), "application/octet-stream", obj, nil)
}

func (h *FileHandler) HandleDeleteFile(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/")

	bucket := h.Store.GetBucket()

	// First check if the file exists (quick operation)
	var err error
	for attempt := range jsRetryAttempts {
		if attempt > 0 {
			time.Sleep(jsRetryDelay)
		}
		_, err = bucket.GetInfo(c.Request.Context(), path)
		if err == nil {
			break
		}
		if !errors.Is(err, jetstream.ErrNoStreamResponse) {
			break
		}
	}
	if err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			log.Warn().Str("path", path).Msg("file not found for deletion")
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		log.Error().Err(err).Str("path", path).Msg("failed to check file existence")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to check file: %v", err)})
		return
	}

	// Return 202 Accepted immediately - delete happens async
	c.Status(http.StatusAccepted)

	// Delete in background goroutine (fire-and-forget)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), h.DeleteDeadline)
		defer cancel()

		if err := bucket.Delete(ctx, path); err != nil {
			log.Error().Err(err).Str("path", path).Msg("async delete failed")
		} else {
			log.Info().Str("path", path).Msg("async delete completed")
		}
	}()
}

// ownerFromTLS extracts the Common Name from the client's mTLS certificate.
func ownerFromTLS(c *gin.Context) string {
	if c.Request.TLS != nil && len(c.Request.TLS.PeerCertificates) > 0 {
		return c.Request.TLS.PeerCertificates[0].Subject.CommonName
	}
	return ""
}
