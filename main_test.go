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
	"time"

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

func TestCORS(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodOptions, "/api/links", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	newTestRouter().ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "http://localhost:5173", response.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete)
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Headers"), "Range")

	request = httptest.NewRequest(http.MethodGet, "/api/links", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response = httptest.NewRecorder()
	newTestRouter().ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Header().Get("Access-Control-Expose-Headers"), "Content-Range")
}

func TestListLinks(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/first", ShortName: "first"})
	response := performRequest(t, newRouter(store, "https://short.test"), http.MethodGet, "/api/links", "")

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `[{"id":1,"original_url":"https://example.com/first","short_name":"first","short_url":"https://short.test/r/first"}]`, response.Body.String())
}

func TestListLinksPagination(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(
		link{ID: 1, OriginalURL: "https://example.com/one", ShortName: "one"},
		link{ID: 2, OriginalURL: "https://example.com/two", ShortName: "two"},
		link{ID: 3, OriginalURL: "https://example.com/three", ShortName: "three"},
		link{ID: 4, OriginalURL: "https://example.com/four", ShortName: "four"},
	)
	router := newRouter(store, "https://short.test")

	for _, testCase := range []struct {
		name         string
		query        string
		contentRange string
		body         string
	}{
		{
			name:         "inclusive range",
			query:        "%5B0%2C1%5D",
			contentRange: "links 0-1/4",
			body:         `[{"id":1,"original_url":"https://example.com/one","short_name":"one","short_url":"https://short.test/r/one"},{"id":2,"original_url":"https://example.com/two","short_name":"two","short_url":"https://short.test/r/two"}]`,
		},
		{
			name:         "range is limited by collection end",
			query:        "%5B2%2C9%5D",
			contentRange: "links 2-3/4",
			body:         `[{"id":3,"original_url":"https://example.com/three","short_name":"three","short_url":"https://short.test/r/three"},{"id":4,"original_url":"https://example.com/four","short_name":"four","short_url":"https://short.test/r/four"}]`,
		},
		{
			name:         "range outside collection",
			query:        "%5B5%2C6%5D",
			contentRange: "links */4",
			body:         `[]`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			response := performRequest(t, router, http.MethodGet, "/api/links?range="+testCase.query, "")

			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, "links", response.Header().Get("Accept-Ranges"))
			assert.Equal(t, testCase.contentRange, response.Header().Get("Content-Range"))
			assert.JSONEq(t, testCase.body, response.Body.String())
		})
	}
}

func TestListLinksRejectsInvalidRange(t *testing.T) {
	t.Parallel()

	router := newTestRouter()
	for _, query := range []string{"%5B0%5D", "%5B2%2C1%5D", "%5B-1%2C0%5D", "not-json"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			response := performRequest(t, router, http.MethodGet, "/api/links?range="+query, "")

			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.JSONEq(t, `{"code":"invalid_range","message":"range must be a JSON array with two integers"}`, response.Body.String())
		})
	}
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

func TestRedirect(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com/long-url", ShortName: "exmpl"})
	router := newRouter(store, "https://short.test")
	request := httptest.NewRequest(http.MethodGet, "/r/exmpl", nil)
	request.Header.Set("CF-Connecting-IP", "203.0.113.7")
	request.Header.Set("User-Agent", "test-agent/1.0")
	request.Header.Set("Referer", "https://example.test/source")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, "https://example.com/long-url", response.Header().Get("Location"))

	visits, err := store.ListLinkVisits(t.Context())
	assert.NoError(t, err)
	assert.Len(t, visits, 1)
	assert.Equal(t, linkVisit{
		ID:        1,
		LinkID:    1,
		IP:        "203.0.113.7",
		UserAgent: "test-agent/1.0",
		Referer:   "https://example.test/source",
		Status:    http.StatusFound,
		CreatedAt: visits[0].CreatedAt,
	}, visits[0])
}

func TestRedirectMissingLink(t *testing.T) {
	t.Parallel()

	response := performRequest(t, newTestRouter(), http.MethodGet, "/r/missing", "")

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.JSONEq(t, `{"code":"link_not_found","message":"link not found"}`, response.Body.String())
}

func TestListLinkVisitsPagination(t *testing.T) {
	t.Parallel()

	store := newMemoryStore(link{ID: 1, OriginalURL: "https://example.com", ShortName: "example"})
	for range 3 {
		_, err := store.CreateLinkVisit(t.Context(), 1, "203.0.113.7", "test-agent", "", http.StatusFound)
		assert.NoError(t, err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/link_visits", nil)
	request.Header.Set("Range", "[1, 9]")
	response := httptest.NewRecorder()
	newRouter(store, "https://short.test").ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "link_visits", response.Header().Get("Accept-Ranges"))
	assert.Equal(t, "link_visits 1-2/3", response.Header().Get("Content-Range"))

	var visits []linkVisitResponse
	assert.NoError(t, json.Unmarshal(response.Body.Bytes(), &visits))
	assert.Len(t, visits, 2)
	assert.Equal(t, int64(2), visits[0].ID)
	assert.Equal(t, int64(3), visits[1].ID)
	assert.Equal(t, int64(1), visits[0].LinkID)
	assert.Equal(t, http.StatusFound, visits[0].Status)
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
	mu          sync.Mutex
	links       map[int64]link
	visits      map[int64]linkVisit
	nextID      int64
	nextVisitID int64
}

func newMemoryStore(links ...link) *memoryStore {
	store := &memoryStore{
		links:  make(map[int64]link, len(links)),
		visits: make(map[int64]linkVisit),
	}
	for _, item := range links {
		store.links[item.ID] = item
		store.nextID = max(store.nextID, item.ID)
	}
	return store
}

func (store *memoryStore) ListLinks(context.Context) ([]link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	return store.orderedLinks(), nil
}

func (store *memoryStore) ListLinksPage(_ context.Context, limit, offset int64) ([]link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	links := store.orderedLinks()
	if offset >= int64(len(links)) {
		return []link{}, nil
	}
	if limit > int64(len(links))-offset {
		limit = int64(len(links)) - offset
	}

	page := make([]link, limit)
	copy(page, links[offset:offset+limit])
	return page, nil
}

func (store *memoryStore) CountLinks(context.Context) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	return int64(len(store.links)), nil
}

func (store *memoryStore) orderedLinks() []link {
	links := make([]link, 0, len(store.links))
	for id := int64(1); id <= store.nextID; id++ {
		if item, ok := store.links[id]; ok {
			links = append(links, item)
		}
	}
	return links
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

func (store *memoryStore) GetLinkByShortName(_ context.Context, shortName string) (link, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, item := range store.links {
		if item.ShortName == shortName {
			return item, nil
		}
	}
	return link{}, sql.ErrNoRows
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

func (store *memoryStore) CreateLinkVisit(_ context.Context, linkID int64, ip, userAgent, referer string, status int) (linkVisit, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.nextVisitID++
	visit := linkVisit{
		ID:        store.nextVisitID,
		LinkID:    linkID,
		IP:        ip,
		UserAgent: userAgent,
		Referer:   referer,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	store.visits[visit.ID] = visit
	return visit, nil
}

func (store *memoryStore) ListLinkVisits(context.Context) ([]linkVisit, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	return store.orderedLinkVisits(), nil
}

func (store *memoryStore) ListLinkVisitsPage(_ context.Context, limit, offset int64) ([]linkVisit, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	visits := store.orderedLinkVisits()
	if offset >= int64(len(visits)) {
		return []linkVisit{}, nil
	}
	if limit > int64(len(visits))-offset {
		limit = int64(len(visits)) - offset
	}

	page := make([]linkVisit, limit)
	copy(page, visits[offset:offset+limit])
	return page, nil
}

func (store *memoryStore) CountLinkVisits(context.Context) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	return int64(len(store.visits)), nil
}

func (store *memoryStore) orderedLinkVisits() []linkVisit {
	visits := make([]linkVisit, 0, len(store.visits))
	for id := int64(1); id <= store.nextVisitID; id++ {
		if visit, ok := store.visits[id]; ok {
			visits = append(visits, visit)
		}
	}
	return visits
}

func (store *memoryStore) shortNameExists(shortName string, exceptID int64) bool {
	for id, item := range store.links {
		if id != exceptID && item.ShortName == shortName {
			return true
		}
	}
	return false
}
