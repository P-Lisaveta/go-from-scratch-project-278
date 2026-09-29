package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// newRouter создаёт и настраивает маршрутизатор со всеми маршрутами.
// Sentry подключается до Recovery, чтобы получать восстановленные паники.
func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(sentrygin.New(sentrygin.Options{}))
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.AbortWithStatus(http.StatusInternalServerError)
	}))

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	router.GET("/debug/sentry", func(c *gin.Context) {
		eventID := sentry.CaptureException(errors.New("тестовая ошибка для проверки Bugsink"))
		if eventID == nil {
			c.String(http.StatusServiceUnavailable, "Sentry не настроен")
			return
		}

		c.String(http.StatusOK, "Событие отправлено: %s", *eventID)
	})

	return router
}

// initSentry настраивает клиент Sentry по переменной окружения SENTRY_DSN.
func initSentry() error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Println("SENTRY_DSN не задан, мониторинг ошибок отключен")
		return nil
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: getEnv("SENTRY_ENVIRONMENT", "production"),
		Release:     getEnv("SENTRY_RELEASE", "go-from-scratch-project-278@1.0.0"),
	}); err != nil {
		return fmt.Errorf("инициализация Sentry: %w", err)
	}

	log.Println("Мониторинг ошибок Sentry включен")
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func run() error {
	if err := initSentry(); err != nil {
		return err
	}
	defer sentry.Flush(2 * time.Second)

	port := getEnv("PORT", "8080")
	if err := newRouter().Run(":" + port); err != nil {
		return fmt.Errorf("запуск сервера: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
