package handlers_test

import (
	"bytes"
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

func setupHandlerTest(t *testing.T) (*gin.Engine, *store.Store, func()) {
	ns := testhelpers.StartEmbeddedNATS(t)

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

	return r, s, func() {
		s.Close()
		ns.Shutdown()
	}
}

func TestHandlePutFile(t *testing.T) {
	r, _, cleanup := setupHandlerTest(t)
	defer cleanup()

	tests := []struct {
		name           string
		path           string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "Valid Upload",
			path:           "/files/test.txt",
			body:           []byte("hello"),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Path (Empty)",
			path:           "/files/",
			body:           []byte("hello"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid Path (Directory)",
			path:           "/files/dir/",
			body:           []byte("hello"),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart body
			body := new(bytes.Buffer)
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", "filename.txt")
			require.NoError(t, err)
			_, err = part.Write(tt.body)
			require.NoError(t, err)
			err = writer.Close()
			require.NoError(t, err)

			req := httptest.NewRequest("PUT", tt.path, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestHandlePutFile_NoFile(t *testing.T) {
	r, _, cleanup := setupHandlerTest(t)
	defer cleanup()

	req := httptest.NewRequest("PUT", "/files/test.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleGetFile(t *testing.T) {
	r, _, cleanup := setupHandlerTest(t)
	defer cleanup()

	// Upload a file first
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("PUT", "/files/test.txt", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(httptest.NewRecorder(), req)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Valid Get",
			path:           "/files/test.txt",
			expectedStatus: http.StatusOK,
			expectedBody:   "content",
		},
		{
			name:           "Not Found",
			path:           "/files/missing.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, tt.expectedBody, w.Body.String())
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=")
			}
		})
	}
}

func TestHandleDeleteFile(t *testing.T) {
	r, _, cleanup := setupHandlerTest(t)
	defer cleanup()

	// Upload a file first
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("content"))
	writer.Close()

	req := httptest.NewRequest("PUT", "/files/test.txt", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(httptest.NewRecorder(), req)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "Valid Delete",
			path:           "/files/test.txt",
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "Delete Non-Existent",
			path:           "/files/missing.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
