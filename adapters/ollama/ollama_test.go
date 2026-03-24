package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbed_Success(t *testing.T) {
	expected := make([]float32, 768)
	for i := range expected {
		expected[i] = float32(i) * 0.001
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/embeddings", r.URL.Path)

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "nomic-embed-text", body["model"])
		assert.Equal(t, "hello world", body["prompt"])

		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": expected,
		})
	}))
	defer srv.Close()

	client := New(srv.URL, "nomic-embed-text", 5*time.Second)
	result, err := client.Embed(context.Background(), "hello world")

	require.NoError(t, err)
	assert.Len(t, result, 768)
	assert.InDelta(t, float64(expected[1]), float64(result[1]), 0.0001)
}

func TestEmbed_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second)
	}))
	defer srv.Close()

	client := New(srv.URL, "nomic-embed-text", 100*time.Millisecond)
	_, err := client.Embed(context.Background(), "hello")

	assert.Error(t, err)
}

func TestEmbed_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := New(srv.URL, "nomic-embed-text", 5*time.Second)
	_, err := client.Embed(context.Background(), "hello")

	assert.Error(t, err)
}

func TestCheckHealth_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]interface{}{"models": []string{}})
	}))
	defer srv.Close()

	client := New(srv.URL, "nomic-embed-text", 5*time.Second)
	err := client.CheckHealth(context.Background())
	assert.NoError(t, err)
}

func TestCheckHealth_Unreachable(t *testing.T) {
	client := New("http://localhost:19999", "nomic-embed-text", 500*time.Millisecond)
	err := client.CheckHealth(context.Background())
	assert.Error(t, err)
}
