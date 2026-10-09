package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestPingRoute(t *testing.T) {
	t.Parallel()

	response := performRequest(t, newTestRouter(), http.MethodGet, "/ping", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "pong", response.Body.String())
}

func TestListLinks(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/first", ShortName: "first"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodGet, "/api/links", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `[{"id":1,"original_url":"https://example.com/first","short_name":"first","short_url":"https://short.test/r/first"}]`, response.Body.String())
}

func TestCreateLink(t *testing.T) {
	t.Parallel()

	response := performRequest(t, newTestRouter(), http.MethodPost, "/api/links", `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.JSONEq(t, `{"id":1,"original_url":"https://example.com/long-url","short_name":"exmpl","short_url":"https://short.test/r/exmpl"}`, response.Body.String())
}

func TestCreateLinkGeneratesShortName(t *testing.T) {
	t.Parallel()

	response := performRequest(t, newTestRouter(), http.MethodPost, "/api/links", `{"original_url":"https://example.com/long-url"}`)

	assert.Equal(t, http.StatusCreated, response.Code)
	var responseLink linkResponse
	assert.NoError(t, json.Unmarshal(response.Body.Bytes(), &responseLink))
	assert.Len(t, responseLink.ShortName, generatedShortNameLength)
	assert.Equal(t, "https://short.test/r/"+responseLink.ShortName, responseLink.ShortURL)
}

func TestGetLink(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/long-url", ShortName: "exmpl"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodGet, "/api/links/1", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"id":1,"original_url":"https://example.com/long-url","short_name":"exmpl","short_url":"https://short.test/r/exmpl"}`, response.Body.String())
}

func TestUpdateLink(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/old-url", ShortName: "old"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodPut, "/api/links/1", `{"original_url":"https://example.com/new-url","short_name":"new"}`)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"id":1,"original_url":"https://example.com/new-url","short_name":"new","short_url":"https://short.test/r/new"}`, response.Body.String())
}

func TestDeleteLink(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/long-url", ShortName: "exmpl"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodDelete, "/api/links/1", "")

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Empty(t, response.Body.String())

	response = performRequest(t, newRouter(store, "https://short.test"), http.MethodGet, "/api/links/1", "")
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestNotFoundErrors(t *testing.T) {
	t.Parallel()

	router := newTestRouter()
	for _, testCase := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/links/404"},
		{method: http.MethodPut, path: "/api/links/404", body: `{"original_url":"https://example.com/long-url","short_name":"exmpl"}`},
		{method: http.MethodDelete, path: "/api/links/404"},
	} {
		t.Run(testCase.method, func(t *testing.T) {
			t.Parallel()
			response := performRequest(t, router, testCase.method, testCase.path, testCase.body)
			assert.Equal(t, http.StatusNotFound, response.Code)
			assert.JSONEq(t, `{"code":"link_not_found","message":"link not found"}`, response.Body.String())
		})
	}
}

func TestDuplicateShortName(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/first", ShortName: "taken"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodPost, "/api/links", `{"original_url":"https://example.com/second","short_name":"taken"}`)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.JSONEq(t, `{"code":"short_name_taken","message":"short_name is already in use"}`, response.Body.String())
}

func newTestRouter() *gin.Engine {
	return newRouter(newMemoryStore(), "https://short.test")
}

func performRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

type memoryStore struct {
	mu     sync.Mutex
	links  map[int64]link
	nextID int64
}

func newMemoryStore(links ...link) *memoryStore {
	store := &memoryStore{links: make(map[int64]link, len(links))}
	for _, item := range links {
		store.links[item.ID] = item
		store.nextID = max(store.nextID, item.ID)
	}
	return store
}

func (store *memoryStore) ListLinks(context.Context) ([]link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	links := make([]link, 0, len(store.links))
	for id := int64(1); id <= store.nextID; id++ {
		if item, ok := store.links[id]; ok {
			links = append(links, item)
		}
	}
	return links, nil
}

func (store *memoryStore) GetLink(_ context.Context, id int64) (link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	item, ok := store.links[id]
	if !ok {
		return link{}, sql.ErrNoRows
	}
	return item, nil
}

func (store *memoryStore) CreateLink(_ context.Context, originalURL, shortName string) (link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if store.shortNameExists(shortName, 0) {
		return link{}, &pgconn.PgError{Code: "23505"}
	}
	store.nextID++
	item := link{ID: store.nextID, OriginalURL: originalURL, ShortName: shortName}
	store.links[item.ID] = item
	return item, nil
}

func (store *memoryStore) UpdateLink(_ context.Context, id int64, originalURL, shortName string) (link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	item, ok := store.links[id]
	if !ok {
		return link{}, sql.ErrNoRows
	}
	if store.shortNameExists(shortName, id) {
		return link{}, &pgconn.PgError{Code: "23505"}
	}
	item.OriginalURL = originalURL
	item.ShortName = shortName
	store.links[item.ID] = item
	return item, nil
}

func (store *memoryStore) DeleteLink(_ context.Context, id int64) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.links[id]; !ok {
		return 0, nil
	}
	delete(store.links, id)
	return 1, nil
}

func (store *memoryStore) shortNameExists(shortName string, exceptID int64) bool {
	for id, item := range store.links {
		if id != exceptID && item.ShortName == shortName {
			return true
		}
	}
	return false
}
