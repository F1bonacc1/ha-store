package handlers_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlerErrors(t *testing.T) {
	r, s, cleanup := setupHandlerTest(t)
	defer cleanup()

	// Close the store connection to simulate NATS errors
	s.Close()

	tests := []struct {
		name           string
		method         string
		path           string
		body           []byte
		expectedStatus int
	}{
		{
			name:           "PutFile NATS Error",
			method:         "PUT",
			path:           "/files/error.txt",
			body:           []byte("data"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "GetFile NATS Error",
			method:         "GET",
			path:           "/files/error.txt",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "DeleteFile NATS Error",
			method:         "DELETE",
			path:           "/files/error.txt",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "PutDir NATS Error",
			method:         "PUT",
			path:           "/dirs/error",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "ListDir NATS Error",
			method:         "GET",
			path:           "/dirs/error",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "DeleteDir NATS Error",
			method:         "DELETE",
			path:           "/dirs/error",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if len(tt.body) > 0 {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("file", "filename")
				part.Write(tt.body)
				writer.Close()

				req = httptest.NewRequest(tt.method, tt.path, body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
