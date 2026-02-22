package router

import (
	"project/internal/transport/rest"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewRouter(h *rest.Handler) *echo.Echo {
	e := echo.New()

	// Полезные мидлвары: логирование запросов и восстановление после паники
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.POST("/api/v1/send", h.SendMessage)

	return e
}
