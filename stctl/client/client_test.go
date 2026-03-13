package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferStats_String(t *testing.T) {
	tests := []struct {
		name     string
		stats    TransferStats
		contains []string
	}{
		{"bytes", TransferStats{Bytes: 500, Duration: time.Second}, []string{"500 B", "1s", "/s"}},
		{"kilobytes", TransferStats{Bytes: 2048, Duration: time.Second}, []string{"2.00 KB", "1s"}},
		{"megabytes", TransferStats{Bytes: 5 * 1024 * 1024, Duration: 2 * time.Second}, []string{"5.00 MB", "2s"}},
		{"gigabytes", TransferStats{Bytes: 2 * 1024 * 1024 * 1024, Duration: 10 * time.Second}, []string{"2.00 GB", "10s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.stats.String()
			for _, c := range tt.contains {
				assert.Contains(t, s, c)
			}
		})
	}
}

func TestPutFile(t *testing.T) {
	var receivedPath string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		receivedBody, err = io.ReadAll(file)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Create a temp file to upload
	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(localFile, []byte("hello world"), 0644))

	c := New(srv.URL)
	stats, err := c.PutFile("docs/test.txt", localFile)
	require.NoError(t, err)

	assert.Equal(t, "/files/docs/test.txt", receivedPath)
	assert.Equal(t, []byte("hello world"), receivedBody)
	assert.Equal(t, int64(11), stats.Bytes)
	assert.Greater(t, stats.Duration.Nanoseconds(), int64(0))
}

func TestPutFile_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "storage failure"})
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(localFile, []byte("data"), 0644))

	c := New(srv.URL)
	_, err := c.PutFile("test.txt", localFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPutFile_FileNotFound(t *testing.T) {
	c := New("http://localhost:0")
	_, err := c.PutFile("test.txt", "/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestGetFile(t *testing.T) {
	content := "file content here"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/docs/test.txt", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	localFile := filepath.Join(tmpDir, "downloaded.txt")

	c := New(srv.URL)
	stats, err := c.GetFile("docs/test.txt", localFile)
	require.NoError(t, err)

	data, err := os.ReadFile(localFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
	assert.Equal(t, int64(17), stats.Bytes)
	assert.Greater(t, stats.Duration.Nanoseconds(), int64(0))
}

func TestGetFile_ToStdout(t *testing.T) {
	content := "stdout content"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer srv.Close()

	// Empty localPath means write to stdout — tested via GetFileToWriter
	c := New(srv.URL)
	var buf strings.Builder
	stats, err := c.GetFileToWriter("test.txt", &buf)
	require.NoError(t, err)
	assert.Equal(t, content, buf.String())
	assert.Equal(t, int64(14), stats.Bytes)
}

func TestGetFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetFile("missing.txt", t.TempDir()+"/out.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestDeleteFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/files/docs/test.txt", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.DeleteFile("docs/test.txt")
	require.NoError(t, err)
}

func TestDeleteFile_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.DeleteFile("missing.txt")
	assert.Error(t, err)
}

func TestCreateDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dirs/mydir", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Empty(t, r.URL.Query().Get("extract"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.CreateDir("mydir")
	require.NoError(t, err)
}

func TestListDir_RootSlash(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{"tmp/"})
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.ListDir("/")
	require.NoError(t, err)
	assert.Equal(t, "/dirs/", gotPath) // no double slash
	assert.Equal(t, []string{"tmp/"}, result)
}

func TestListDir(t *testing.T) {
	files := []string{"mydir/a.txt", "mydir/b.txt", "mydir/sub/c.txt"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dirs/mydir", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.ListDir("mydir")
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt", "b.txt", "sub/"}, result)
}

func TestListDir_Root(t *testing.T) {
	files := []string{"tmp/", "tmp/t/", "docs/readme.txt"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.ListDir("/")
	require.NoError(t, err)
	assert.Equal(t, []string{"docs/", "tmp/"}, result)
}

func TestListDir_Nested(t *testing.T) {
	files := []string{"a/b/c.txt", "a/b/d.txt", "a/b/sub/e.txt"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.ListDir("a/b")
	require.NoError(t, err)
	assert.Equal(t, []string{"c.txt", "d.txt", "sub/"}, result)
}

func TestListDir_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
	}))
	defer srv.Close()

	c := New(srv.URL)
	result, err := c.ListDir("empty")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestDeleteDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dirs/mydir", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	err := c.DeleteDir("mydir")
	require.NoError(t, err)
}

func TestExtractArchive(t *testing.T) {
	var receivedPath string
	var receivedExtract string
	var receivedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedExtract = r.URL.Query().Get("extract")
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()
		receivedBody, err = io.ReadAll(file)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	archive := filepath.Join(tmpDir, "test.tgz")
	require.NoError(t, os.WriteFile(archive, []byte("fake-archive-data"), 0644))

	c := New(srv.URL)
	err := c.ExtractArchive("mydir", archive, "tgz")
	require.NoError(t, err)

	assert.Equal(t, "/dirs/mydir", receivedPath)
	assert.Equal(t, "tgz", receivedExtract)
	assert.Equal(t, []byte("fake-archive-data"), receivedBody)
}

func TestExtractArchive_InvalidType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported extract type"})
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	archive := filepath.Join(tmpDir, "test.rar")
	require.NoError(t, os.WriteFile(archive, []byte("data"), 0644))

	c := New(srv.URL)
	err := c.ExtractArchive("mydir", archive, "rar")
	assert.Error(t, err)
}
