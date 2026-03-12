//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/f1bonacc1/ha-store/handlers"
	"github.com/f1bonacc1/ha-store/store"
	"github.com/f1bonacc1/ha-store/testhelpers"
	"github.com/nats-io/nats.go"
)

func setupIntegrationServer(t *testing.T) (*gin.Engine, *store.Store) {
	natsURL := os.Getenv("TEST_NATS_URL")
	var nc *nats.Conn
	var err error

	if natsURL != "" {
		// Connect to external NATS
		nc, err = nats.Connect(natsURL)
		require.NoError(t, err, "Failed to connect to external NATS")
	} else {
		// Use embedded NATS
		ns := testhelpers.StartEmbeddedNATS(t)
		t.Cleanup(func() { ns.Shutdown() })
		nc = testhelpers.ConnectNATS(t, ns)
	}

	// Create JetStream Context
	_, err = nc.JetStream()
	require.NoError(t, err)

	url := natsURL
	if url == "" {
		url = nc.ConnectedUrl()
	}

	s, err := store.New(url, 1)
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

func TestIntegration_FileLifecycle(t *testing.T) {
	r, s := setupIntegrationServer(t)
	defer s.Close()

	filename := fmt.Sprintf("integration-test-%d.txt", time.Now().UnixNano())
	content := []byte("integration test content")

	// 1. Upload File
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write(content)
	writer.Close()

	req, _ := http.NewRequest("PUT", "/files/"+filename, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Download File
	req, _ = http.NewRequest("GET", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.Bytes())
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	// 3. Delete File (async - returns 202 Accepted)
	req, _ = http.NewRequest("DELETE", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Wait briefly for async delete to complete
	time.Sleep(100 * time.Millisecond)

	// 4. Verify Deletion
	req, _ = http.NewRequest("GET", "/files/"+filename, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestIntegration_DirectoryListing(t *testing.T) {
	r, s := setupIntegrationServer(t)
	defer s.Close()

	prefix := fmt.Sprintf("integration-dir-%d", time.Now().UnixNano())
	file1 := prefix + "/file1.txt"
	file2 := prefix + "/sub/file2.txt"

	// Upload files
	for _, f := range []string{file1, file2} {
		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, _ := writer.CreateFormFile("file", "test.txt")
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
