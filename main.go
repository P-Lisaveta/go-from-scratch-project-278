package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/P-Lisaveta/go-from-scratch-project-278/internal/db/sqlc"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	databaseTimeout           = 5 * time.Second
	generatedShortNameLength  = 8
	generatedShortNameRetries = 5
)

var shortNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type linkStore interface {
	ListLinks(context.Context) ([]link, error)
	GetLink(context.Context, int64) (link, error)
	CreateLink(context.Context, string, string) (link, error)
	UpdateLink(context.Context, int64, string, string) (link, error)
	DeleteLink(context.Context, int64) (int64, error)
}

type api struct {
	store   linkStore
	baseURL string
}

type linkInput struct {
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
}

type link struct {
	ID          int64
	OriginalURL string
	ShortName   string
}

type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type databaseStore struct {
	queries *sqlc.Queries
}

func newDatabaseStore(queries *sqlc.Queries) *databaseStore {
	return &databaseStore{queries: queries}
}

func (store databaseStore) ListLinks(ctx context.Context) ([]link, error) {
	rows, err := store.queries.ListLinks(ctx)
	if err != nil {
		return nil, err
	}

	links := make([]link, 0, len(rows))
	for _, row := range rows {
		links = append(links, link{ID: row.ID, OriginalURL: row.OriginalUrl, ShortName: row.ShortName})
	}
	return links, nil
}

func (store databaseStore) GetLink(ctx context.Context, id int64) (link, error) {
	row, err := store.queries.GetLink(ctx, id)
	return link{ID: row.ID, OriginalURL: row.OriginalUrl, ShortName: row.ShortName}, err
}

func (store databaseStore) CreateLink(ctx context.Context, originalURL, shortName string) (link, error) {
	row, err := store.queries.CreateLink(ctx, sqlc.CreateLinkParams{
		OriginalUrl: originalURL,
		ShortName:   shortName,
	})
	return link{ID: row.ID, OriginalURL: row.OriginalUrl, ShortName: row.ShortName}, err
}

func (store databaseStore) UpdateLink(ctx context.Context, id int64, originalURL, shortName string) (link, error) {
	row, err := store.queries.UpdateLink(ctx, sqlc.UpdateLinkParams{
		ID:          id,
		OriginalUrl: originalURL,
		ShortName:   shortName,
	})
	return link{ID: row.ID, OriginalURL: row.OriginalUrl, ShortName: row.ShortName}, err
}

func (store databaseStore) DeleteLink(ctx context.Context, id int64) (int64, error) {
	return store.queries.DeleteLink(ctx, id)
}

// newRouter creates an HTTP router for the short-links API.
func newRouter(store linkStore, baseURL string) *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(sentrygin.New(sentrygin.Options{}))
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.AbortWithStatus(http.StatusInternalServerError)
	}))

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	router.GET("/debug/sentry", func(c *gin.Context) {
		eventID := sentry.CaptureException(errors.New("test error for Bugsink verification"))
		if eventID == nil {
			c.String(http.StatusServiceUnavailable, "Sentry is not configured")
			return
		}

		c.String(http.StatusOK, "event sent: %s", *eventID)
	})

	api := api{store: store, baseURL: strings.TrimRight(baseURL, "/")}
	router.GET("/api/links", api.listLinks)
	router.POST("/api/links", api.createLink)
	router.GET("/api/links/:id", api.getLink)
	router.PUT("/api/links/:id", api.updateLink)
	router.DELETE("/api/links/:id", api.deleteLink)

	return router
}

func (api api) listLinks(c *gin.Context) {
	ctx, cancel := requestContext(c)
	defer cancel()

	links, err := api.store.ListLinks(ctx)
	if err != nil {
		writeStoreError(c, err)
		return
	}

	response := make([]linkResponse, 0, len(links))
	for _, link := range links {
		response = append(response, api.toLinkResponse(link))
	}

	c.JSON(http.StatusOK, response)
}

func (api api) createLink(c *gin.Context) {
	input, ok := readLinkInput(c, false)
	if !ok {
		return
	}

	ctx, cancel := requestContext(c)
	defer cancel()

	link, err := api.create(ctx, input)
	if err != nil {
		writeStoreError(c, err)
		return
	}

	c.JSON(http.StatusCreated, api.toLinkResponse(link))
}

func (api api) getLink(c *gin.Context) {
	id, ok := readLinkID(c)
	if !ok {
		return
	}

	ctx, cancel := requestContext(c)
	defer cancel()

	link, err := api.store.GetLink(ctx, id)
	if err != nil {
		writeStoreError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.toLinkResponse(link))
}

func (api api) updateLink(c *gin.Context) {
	id, ok := readLinkID(c)
	if !ok {
		return
	}

	input, ok := readLinkInput(c, true)
	if !ok {
		return
	}

	ctx, cancel := requestContext(c)
	defer cancel()

	link, err := api.store.UpdateLink(ctx, id, input.OriginalURL, input.ShortName)
	if err != nil {
		writeStoreError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.toLinkResponse(link))
}

func (api api) deleteLink(c *gin.Context) {
	id, ok := readLinkID(c)
	if !ok {
		return
	}

	ctx, cancel := requestContext(c)
	defer cancel()

	rows, err := api.store.DeleteLink(ctx, id)
	if err != nil {
		writeStoreError(c, err)
		return
	}
	if rows == 0 {
		writeError(c, http.StatusNotFound, "link_not_found", "link not found")
		return
	}

	c.Status(http.StatusNoContent)
}

func (api api) create(ctx context.Context, input linkInput) (link, error) {
	if input.ShortName != "" {
		return api.store.CreateLink(ctx, input.OriginalURL, input.ShortName)
	}

	for range generatedShortNameRetries {
		shortName, err := generateShortName()
		if err != nil {
			return link{}, err
		}

		link, err := api.store.CreateLink(ctx, input.OriginalURL, shortName)
		if !isUniqueViolation(err) {
			return link, err
		}
	}

	return link{}, errors.New("failed to generate a unique short name")
}

func (api api) toLinkResponse(link link) linkResponse {
	return linkResponse{
		ID:          link.ID,
		OriginalURL: link.OriginalURL,
		ShortName:   link.ShortName,
		ShortURL:    api.baseURL + "/r/" + link.ShortName,
	}
}

func readLinkInput(c *gin.Context, requireShortName bool) (linkInput, bool) {
	var input linkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return linkInput{}, false
	}

	input.OriginalURL = strings.TrimSpace(input.OriginalURL)
	input.ShortName = strings.TrimSpace(input.ShortName)
	if err := validateLinkInput(input, requireShortName); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return linkInput{}, false
	}

	return input, true
}

func validateLinkInput(input linkInput, requireShortName bool) error {
	parsedURL, err := url.ParseRequestURI(input.OriginalURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return errors.New("original_url must be a valid http or https URL")
	}
	if requireShortName && input.ShortName == "" {
		return errors.New("short_name is required")
	}
	if input.ShortName != "" && !shortNamePattern.MatchString(input.ShortName) {
		return errors.New("short_name must contain 1 to 64 letters, digits, hyphens, or underscores")
	}

	return nil
}

func readLinkID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return 0, false
	}

	return id, true
}

func requestContext(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), databaseTimeout)
}

func writeStoreError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(c, http.StatusNotFound, "link_not_found", "link not found")
		return
	}
	if isUniqueViolation(err) {
		writeError(c, http.StatusBadRequest, "short_name_taken", "short_name is already in use")
		return
	}

	writeError(c, http.StatusInternalServerError, "internal_error", "internal server error")
}

func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == "23505"
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Code: code, Message: message})
}

func generateShortName() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, generatedShortNameLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}

	return string(bytes), nil
}

// initSentry configures Sentry when SENTRY_DSN is set.
func initSentry() error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Println("SENTRY_DSN is not set, error monitoring is disabled")
		return nil
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: getEnv("SENTRY_ENVIRONMENT", "production"),
		Release:     getEnv("SENTRY_RELEASE", "go-from-scratch-project-278@1.0.0"),
	}); err != nil {
		return fmt.Errorf("initialize Sentry: %w", err)
	}

	log.Println("error monitoring is enabled")
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func openDatabase(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingContext, cancel := context.WithTimeout(ctx, databaseTimeout)
	defer cancel()
	if err := db.PingContext(pingContext); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func run() error {
	if err := initSentry(); err != nil {
		return err
	}
	defer sentry.Flush(2 * time.Second)

	port := getEnv("PORT", "8080")
	db, err := openDatabase(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()

	baseURL := getEnv("BASE_URL", "http://localhost:"+port)
	if err := newRouter(newDatabaseStore(sqlc.New(db)), baseURL).Run(":" + port); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
