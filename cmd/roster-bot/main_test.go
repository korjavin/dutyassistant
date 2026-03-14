package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateHTTPServer(t *testing.T) {
	handler := http.NewServeMux()
	srv := createHTTPServer(handler)

	assert.NotNil(t, srv)
	assert.Equal(t, ":8080", srv.Addr)
	assert.Equal(t, handler, srv.Handler)
	assert.Equal(t, 10*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, srv.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)
	assert.Equal(t, 1<<20, srv.MaxHeaderBytes)
}
