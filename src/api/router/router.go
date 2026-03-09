package router

import (
	"fmt"
	"os"
	startflags "project/internal/infrastructure/start_flags"
	"project/internal/transport/rest"
	"slices"

	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewRouter(h *rest.Handler, enabledServices []string) *echo.Echo {
	e := echo.New()

	e.GET("/health", func(c echo.Context) error {
		return c.NoContent(200)
	})

	// Полезные мидлвары: логирование запросов и восстановление после паники
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	jwtConfig := echojwt.Config{
		SigningKey: []byte(os.Getenv("JWT_SECRET")),
		ContextKey: "user",
		// Ищем в заголовоке "Authorization" ИЛИ в query-параметре "token"
		TokenLookup: "header:Authorization:Bearer ,query:token",
	}

	v1 := e.Group("/api/v1")

	enabled := func(name startflags.Flag) bool {
		ret := slices.Contains(enabledServices, string(name)) || slices.Contains(enabledServices, "all")
		if ret {
			e.Logger.Info(fmt.Sprintf("API %s start", string(name)))
		}
		return ret
	}

	if enabled(startflags.UserRegisterFlag) {
		v1.POST("/register_user", h.RegisterUser)
	}

	if enabled(startflags.UserLoginFlag) {
		v1.POST("/login_user", h.LoginUser)
	}

	protected := v1.Group("/prot")
	protected.Use(echojwt.WithConfig(jwtConfig))

	if enabled(startflags.SearchUserFlag) {
		protected.GET("/search_user", h.SearchUser)
	}
	if enabled(startflags.SendMessage) {
		protected.POST("/send_message", h.SendMessage)
	}
	if enabled(startflags.GetMessages) {
		// Внутри NewRouter, в группе protected
		protected.POST("/join_chat", h.JoinChat)
		protected.GET("/messages", h.GetChatHistory)
		protected.GET("/ws_messages", h.HandleWS)
	}
	if enabled(startflags.ChangeProfile) {
		protected.GET("/get_profile", h.GetProfile)
		protected.POST("/change_profile", h.ChangeProfile)
	}
	if enabled(startflags.ChangeProfileAvatar) {
		protected.GET("/change_profile_avatar_url", h.GetUploadURL)
		protected.POST("/confirm_change_profile_avatar", h.ConfirmUpload)
	}

	return e
}
