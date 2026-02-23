package router

import (
	"project/internal/transport/rest"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewRouter(h *rest.Handler, enabledServices string) *echo.Echo {
	e := echo.New()

	// Полезные мидлвары: логирование запросов и восстановление после паники
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	v1 := e.Group("/api/v1")

	//e.POST("/api/v1/send", h.SendMessage)
	if enabledServices == "users" || enabledServices == "all" {
		v1.POST("/register_user", h.RegisterUser)
		// v1.POST("/login", h.Login)
	}

	if enabledServices == "billing" || enabledServices == "all" {
		// v1.POST("/pay", h.ProcessPayment)
	}

	return e
}
