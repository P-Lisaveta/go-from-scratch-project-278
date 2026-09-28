package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	// Логгирование и восстановление после паник.
	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	// Запускаем приложение на порту 8080.
	router.Run(":8080")
}
