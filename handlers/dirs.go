package handlers

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

func (h *FileHandler) HandlePutDir(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid directory path"})
		return
	}

	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Read optional metadata form fields for extracted files
	defaults := metaDefaults(c)

	extractType := c.Query("extract")
	if extractType == "" {
		// Existing behavior: Create empty directory marker
		bucket := h.Store.GetBucket()
		_, err := bucket.PutBytes(c.Request.Context(), path, []byte{})
		if err != nil {
			log.Error().Err(err).Str("path", path).Msg("failed to create directory")
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create directory: %v", err)})
			return
		}
		c.Status(http.StatusOK)
		return
	}

	// Archive upload handling
	file, err := c.FormFile("file")
	if err != nil {
		log.Error().Err(err).Msg("failed to get file from form")
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to get file from form"})
		return
	}

	// We need a seekable reader for zip and 7z, so we save to a temp file
	tmpFile, err := os.CreateTemp("", "ha-store-upload-*")
	if err != nil {
		log.Error().Err(err).Msg("failed to create temp file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process upload"})
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up
	defer tmpFile.Close()

	src, err := file.Open()
	if err != nil {
		log.Error().Err(err).Msg("failed to open uploaded file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
		return
	}
	defer src.Close()

	if _, err := io.Copy(tmpFile, src); err != nil {
		log.Error().Err(err).Msg("failed to save uploaded file to temp")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process upload"})
		return
	}

	// Re-open temp file for reading
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		log.Error().Err(err).Msg("failed to open temp file")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process upload"})
		return
	}
	defer f.Close()

	fileSize := file.Size
	var extractErr error

	switch extractType {
	case "zip":
		extractErr = h.extractZip(c.Request.Context(), f, fileSize, path, defaults)
	case "targz", "tgz":
		extractErr = h.extractTarGz(c.Request.Context(), f, path, defaults)
	case "zst":
		extractErr = h.extractTarZst(c.Request.Context(), f, path, defaults)
	case "7z", "7zip":
		extractErr = h.extract7z(c.Request.Context(), f, fileSize, path, defaults)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported extract type: " + extractType})
		return
	}

	if extractErr != nil {
		log.Error().Err(extractErr).Str("type", extractType).Msg("failed to extract archive")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to extract archive: %v", extractErr)})
		return
	}

	c.Status(http.StatusOK)
}

func (h *FileHandler) extractZip(ctx context.Context, r io.ReaderAt, size int64, destPath string, defaults map[string]string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}

	bucket := h.Store.GetBucket()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destPath, f.Name)
		// Ensure standardized path separators
		targetPath = strings.ReplaceAll(targetPath, "\\", "/")

		throttled := NewThrottledReader(rc, h.ThrottleSpeed)

		meta := jetstream.ObjectMeta{
			Name:     targetPath,
			Metadata: copyMeta(defaults),
			Opts:     &jetstream.ObjectMetaOptions{ChunkSize: 1024 * 1024},
		}

		_, err = bucket.Put(ctx, meta, throttled)
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to put file %s: %w", f.Name, err)
		}
	}
	return nil
}

func (h *FileHandler) extractTarGz(ctx context.Context, r io.Reader, destPath string, defaults map[string]string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	return h.extractTar(ctx, gzr, destPath, defaults)
}

func (h *FileHandler) extractTarZst(ctx context.Context, r io.Reader, destPath string, defaults map[string]string) error {
	zsr, err := zstd.NewReader(r)
	if err != nil {
		return err
	}
	defer zsr.Close()

	return h.extractTar(ctx, zsr, destPath, defaults)
}

func (h *FileHandler) extractTar(ctx context.Context, r io.Reader, destPath string, defaults map[string]string) error {
	tr := tar.NewReader(r)
	bucket := h.Store.GetBucket()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag == tar.TypeDir {
			continue
		}

		targetPath := filepath.Join(destPath, header.Name)
		targetPath = strings.ReplaceAll(targetPath, "\\", "/")

		throttled := NewThrottledReader(tr, h.ThrottleSpeed)

		// Use tar header metadata when available, fall back to defaults
		m := copyMeta(defaults)
		if header.Mode != 0 {
			m["permissions"] = fmt.Sprintf("%04o", header.Mode&0o7777)
		}
		if header.Uname != "" {
			m["owner"] = header.Uname
		}
		if header.Gname != "" {
			m["group"] = header.Gname
		}

		meta := jetstream.ObjectMeta{
			Name:     targetPath,
			Metadata: m,
			Opts:     &jetstream.ObjectMetaOptions{ChunkSize: 1024 * 1024},
		}

		_, err = bucket.Put(ctx, meta, throttled)
		if err != nil {
			return fmt.Errorf("failed to put file %s: %w", header.Name, err)
		}
	}
	return nil
}

func (h *FileHandler) extract7z(ctx context.Context, r io.ReaderAt, size int64, destPath string, defaults map[string]string) error {
	zr, err := sevenzip.NewReader(r, size)
	if err != nil {
		return err
	}

	bucket := h.Store.GetBucket()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destPath, f.Name)
		targetPath = strings.ReplaceAll(targetPath, "\\", "/")

		throttled := NewThrottledReader(rc, h.ThrottleSpeed)

		meta := jetstream.ObjectMeta{
			Name:     targetPath,
			Metadata: copyMeta(defaults),
			Opts:     &jetstream.ObjectMetaOptions{ChunkSize: 1024 * 1024},
		}

		_, err = bucket.Put(ctx, meta, throttled)
		rc.Close()
		if err != nil {
			return fmt.Errorf("failed to put file %s: %w", f.Name, err)
		}
	}
	return nil
}

func (h *FileHandler) HandleListDir(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/")

	// Ensure path ends with slash for listing, unless it's root
	if path != "" && !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	bucket := h.Store.GetBucket()

	// List all objects
	infos, err := bucket.List(c.Request.Context())
	if err != nil {
		if err == jetstream.ErrNoObjectsFound {
			if c.Query("detail") == "true" {
				c.JSON(http.StatusOK, []FileInfo{})
			} else {
				c.JSON(http.StatusOK, []string{})
			}
			return
		}
		log.Error().Err(err).Str("path", path).Msg("failed to list directory")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list directory: %v", err)})
		return
	}

	if c.Query("detail") == "true" {
		var files []FileInfo
		for _, info := range infos {
			if strings.HasPrefix(info.Name, path) {
				files = append(files, fileInfoFromObject(info))
			}
		}
		c.JSON(http.StatusOK, files)
		return
	}

	var files []string
	for _, info := range infos {
		// Filter by prefix
		if strings.HasPrefix(info.Name, path) {
			files = append(files, info.Name)
		}
	}

	c.JSON(http.StatusOK, files)
}

func (h *FileHandler) HandleDeleteDir(c *gin.Context) {
	path := c.Param("path")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete root directory"})
		return
	}

	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	bucket := h.Store.GetBucket()

	infos, err := bucket.List(c.Request.Context())
	if err != nil {
		if err == jetstream.ErrNoObjectsFound {
			c.Status(http.StatusOK)
			return
		}
		log.Error().Err(err).Str("path", path).Msg("failed to list directory for deletion")
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list directory for deletion: %v", err)})
		return
	}

	for _, info := range infos {
		if strings.HasPrefix(info.Name, path) {
			if err := bucket.Delete(c.Request.Context(), info.Name); err != nil {
				log.Error().Err(err).Str("path", path).Str("file", info.Name).Msg("failed to delete file")
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to delete file %s: %v", info.Name, err)})
				return
			}
		}
	}

	c.Status(http.StatusOK)
}

// metaDefaults reads optional metadata form fields and returns defaults.
func metaDefaults(c *gin.Context) map[string]string {
	permissions := c.PostForm("permissions")
	if permissions == "" {
		permissions = "0644"
	}
	owner := c.PostForm("owner")
	if owner == "" {
		owner = ownerFromTLS(c)
	}
	group := c.PostForm("group")
	return map[string]string{
		"permissions": permissions,
		"owner":       owner,
		"group":       group,
	}
}

// copyMeta returns a shallow copy of the metadata map.
func copyMeta(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// fileInfoFromObject builds a FileInfo from a NATS ObjectInfo.
func fileInfoFromObject(info *jetstream.ObjectInfo) FileInfo {
	permissions := "0644"
	var owner, group string
	if info.Metadata != nil {
		if v, ok := info.Metadata["permissions"]; ok && v != "" {
			permissions = v
		}
		owner = info.Metadata["owner"]
		group = info.Metadata["group"]
	}
	return FileInfo{
		Name:        info.Name,
		Size:        info.Size,
		ModTime:     info.ModTime.Format(time.RFC3339),
		Permissions: permissions,
		Owner:       owner,
		Group:       group,
	}
}
