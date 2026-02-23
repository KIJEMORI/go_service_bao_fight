package rest

type MessageHandler interface {
	Process(messages []models.Message) error
}

func (h *Handler) SendMessage(c echo.Context) error {
	ctx := c.Request().Context()

	var input struct {
		Text string `json:"text" validate:"required"`
	}

	// Парсим входящий JSON
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if input.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "text is required"})
	}

	// Отправка в Kafka
	err := h.KafkaWriter.WriteMessages(ctx,
		kafka.Message{Value: []byte(input.Text)},
	)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "sent",
		"data":   input.Text,
	})
}
