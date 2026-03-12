package main_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/f1bonacc1/ha-store/handlers"
	"github.com/f1bonacc1/ha-store/store"
	"github.com/f1bonacc1/ha-store/testhelpers"
)

// SetupTestServer creates a new gin engine and store for testing
func SetupTestServer(t *testing.T) (*gin.Engine, *store.Store) {
	// Start embedded NATS
	ns := testhelpers.StartEmbeddedNATS(t)
	// Ensure server is stopped after test
	t.Cleanup(func() {
		ns.Shutdown()
	})

	s, err := store.New(ns.ClientURL(), 1)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	fileHandler := &handlers.FileHandler{
		Store:          s,
		ThrottleSpeed:  150 * 1024 * 1024, // 150 MB/s
		UploadDeadline: 600 * time.Second, // 10 minutes
		DeleteDeadline: 60 * time.Second,  // 1 minute
	}
	r.PUT("/files/*path", fileHandler.HandlePutFile)
	r.GET("/files/*path", fileHandler.HandleGetFile)
	r.DELETE("/files/*path", fileHandler.HandleDeleteFile)
	r.PUT("/dirs/*path", fileHandler.HandlePutDir)
	r.GET("/dirs/*path", fileHandler.HandleListDir)
	r.DELETE("/dirs/*path", fileHandler.HandleDeleteDir)

	return r, s
}

func TestFileLifecycle(t *testing.T) {
	r, s := SetupTestServer(t)
	defer s.Close()

	filename := fmt.Sprintf("testfile-%d.txt", time.Now().UnixNano())
	content := []byte("hello world")

	// 1. Upload File
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "filename.txt")
	part.Write(content)
	writer.Close()

	req, _ := http.NewRequest("PUT", "/files/"+filename, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Get File
	req, _ = http.NewRequest("GET", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.Bytes())

	// 3. Delete File (async - returns 202 Accepted)
	req, _ = http.NewRequest("DELETE", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Wait briefly for async delete to complete
	time.Sleep(100 * time.Millisecond)

	// 4. Get File - Should be 404
	req, _ = http.NewRequest("GET", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDirectoryListing(t *testing.T) {
	r, s := SetupTestServer(t)
	defer s.Close()

	prefix := fmt.Sprintf("dir-%d", time.Now().UnixNano())
	file1 := prefix + "/file1.txt"
	file2 := prefix + "/sub/file2.txt"

	// Upload files
	for _, f := range []string{file1, file2} {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "filename")
		part.Write([]byte("content"))
		writer.Close()

		req, _ := http.NewRequest("PUT", "/files/"+f, body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		r.ServeHTTP(httptest.NewRecorder(), req)
	}

	// List Directory
	req, _ := http.NewRequest("GET", "/dirs/"+prefix, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), file1)
	assert.Contains(t, w.Body.String(), file2)
}
