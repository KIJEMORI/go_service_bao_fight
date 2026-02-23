package server

import (
	"os"
	"project/api/router"              // Импорт вашего роутера
	"project/internal/transport/rest" // Импорт обработчиков
)

// Server структура может хранить зависимости сервера
type Server struct {
	handler *rest.Handler
}

// NewServer создает новый экземпляр сервера
func NewServer(h *rest.Handler) *Server {
	return &Server{handler: h}
}

// Run запускает Echo сервер
func (s *Server) Run(mode string) error {
	// Вызываем функцию NewRouter
	e := router.NewRouter(s.handler, mode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return e.Start(":" + port)
}
