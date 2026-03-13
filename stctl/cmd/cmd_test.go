package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCmd(t *testing.T, srvURL string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append([]string{"--server", srvURL}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestFilePutCmd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpFile := filepath.Join(t.TempDir(), "upload.txt")
	require.NoError(t, os.WriteFile(tmpFile, []byte("data"), 0644))

	out, err := runCmd(t, srv.URL, "file", "put", "docs/upload.txt", tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "/files/docs/upload.txt", gotPath)
	assert.Contains(t, out, "Uploaded docs/upload.txt")
	assert.Contains(t, out, "4 B")
}

func TestFileGetCmd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file content"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.txt")
	out, err := runCmd(t, srv.URL, "file", "get", "docs/test.txt", dest)
	require.NoError(t, err)
	assert.Contains(t, out, "Downloaded docs/test.txt")
	assert.Contains(t, out, "12 B")

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "file content", string(data))
}

func TestFileRmCmd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	out, err := runCmd(t, srv.URL, "file", "rm", "docs/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "/files/docs/test.txt", gotPath)
	assert.Contains(t, out, "Deleted docs/test.txt")
}

func TestDirCreateCmd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := runCmd(t, srv.URL, "dir", "create", "mydir")
	require.NoError(t, err)
	assert.Equal(t, "/dirs/mydir", gotPath)
	assert.Contains(t, out, "Created mydir")
}

func TestDirLsCmd(t *testing.T) {
	files := []string{"mydir/a.txt", "mydir/b.txt", "mydir/sub/c.txt"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}))
	defer srv.Close()

	out, err := runCmd(t, srv.URL, "ls", "mydir")
	require.NoError(t, err)
	assert.Contains(t, out, "a.txt")
	assert.Contains(t, out, "b.txt")
	assert.Contains(t, out, "sub/")
	assert.NotContains(t, out, "mydir/")
}

func TestDirRmCmd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := runCmd(t, srv.URL, "dir", "rm", "mydir")
	require.NoError(t, err)
	assert.Equal(t, "/dirs/mydir", gotPath)
	assert.Contains(t, out, "Deleted mydir")
}

func TestDirExtractCmd(t *testing.T) {
	var gotPath, gotExtract string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotExtract = r.URL.Query().Get("extract")
		file, _, err := r.FormFile("file")
		if err == nil {
			defer file.Close()
			gotBody, _ = io.ReadAll(file)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	archive := filepath.Join(t.TempDir(), "test.tgz")
	require.NoError(t, os.WriteFile(archive, []byte("archive-data"), 0644))

	out, err := runCmd(t, srv.URL, "dir", "extract", "mydir", archive, "--type", "zip")
	require.NoError(t, err)
	assert.Equal(t, "/dirs/mydir", gotPath)
	assert.Equal(t, "zip", gotExtract)
	assert.Equal(t, []byte("archive-data"), gotBody)
	assert.Contains(t, out, "Extracted")
}

func TestDirExtractCmd_DefaultType(t *testing.T) {
	var gotExtract string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotExtract = r.URL.Query().Get("extract")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	archive := filepath.Join(t.TempDir(), "test.tgz")
	require.NoError(t, os.WriteFile(archive, []byte("data"), 0644))

	_, err := runCmd(t, srv.URL, "dir", "extract", "mydir", archive)
	require.NoError(t, err)
	assert.Equal(t, "tgz", gotExtract)
}

func TestMissingArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"file put no args", []string{"file", "put"}},
		{"file get no args", []string{"file", "get"}},
		{"file rm no args", []string{"file", "rm"}},
		{"dir create no args", []string{"dir", "create"}},
		{"ls no args", []string{"ls"}},
		{"dir rm no args", []string{"dir", "rm"}},
		{"dir extract no args", []string{"dir", "extract"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runCmd(t, "http://localhost:0", tt.args...)
			assert.Error(t, err)
		})
	}
}
