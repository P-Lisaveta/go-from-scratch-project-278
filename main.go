package main

import (
	"github.com/gin-gonic/gin"
)

// newRouter создаёт и настраивает маршрутизатор с middleware
// (логгирование и восстановление после паник) и всеми маршрутами.
func newRouter() *gin.Engine {
	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	return router
}

func main() {
	router := newRouter()

	// Запускаем приложение на порту 8080.
	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
