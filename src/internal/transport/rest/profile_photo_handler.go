package rest

import (
	"fmt"
	"net/http"
	"project/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// func (h *Handler) UploadPhoto(c echo.Context) error {
// 	userVal := c.Get("user")
// 	userID, err := getSenderID(c, h, userVal)
// 	if err != nil {
// 		return err
// 	}

// 	file, err := c.FormFile("photo")
// 	if err != nil {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file is required"})
// 	}

// 	src, err := file.Open()
// 	if err != nil {
// 		return err
// 	}
// 	defer src.Close()

// 	// Ограничиваем формат (только картинки)
// 	if !strings.HasSuffix(file.Filename, ".jpg") && !strings.HasSuffix(file.Filename, ".png") {
// 		return c.JSON(http.StatusBadRequest, map[string]string{"error": "only jpg/png allowed"})
// 	}

// 	filename := userID + "_" + time.Now().Format("20060102150405") + ".jpg"
// 	filepath := "./uploads/" + filename

// 	// Сохраняем файл на диск
// 	dst, err := os.Create(filepath)
// 	if err != nil {
// 		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
// 	}
// 	defer dst.Close()

// 	if _, err = io.Copy(dst, src); err != nil {
// 		return err
// 	}

// 	// Обновляем photo_url в БД профиля
// 	photoURL := "/uploads/" + filename
// 	err = h.DB.Model(&models.Profile{}).Where("user_id = ?", userID).Update("photo_url", photoURL).Error
// 	if err != nil {
// 		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database error"})
// 	}

// 	return c.JSON(http.StatusOK, map[string]string{
// 		"status":    "uploaded",
// 		"photo_url": photoURL,
// 	})
// }

func (h *Handler) GetUploadURL(c echo.Context) error {
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	// Генерируем УНИКАЛЬНОЕ имя (ID + время), чтобы сохранить историю в S3
	// Теперь в бакете будет avatars/user1_171024.jpg, avatars/user1_181024.jpg и т.д.
	uniqueSuffix := time.Now().Format("20060102150405")
	objectName := fmt.Sprintf("avatars/%s_%s.jpg", userID, uniqueSuffix)

	// Генерируем ссылку для PUT-запроса
	expiry := time.Second * 60 * 15
	presignedURL, err := h.s3Client.PresignedPutObject(c.Request().Context(),
		"profiles", objectName, expiry)

	if err != nil {
		h.logger.Error("S3 presign failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to gen url"})
	}
	externalURL := strings.Replace(presignedURL.String(), "http://minio:9000/profiles", "http://localhost/s3-upload", 1)

	return c.JSON(http.StatusOK, map[string]string{
		"upload_url": externalURL,
		"object_key": objectName,
	})
}

func (h *Handler) ConfirmUpload(c echo.Context) error {
	var input struct {
		ObjectKey string `json:"object_key"`
	}
	c.Bind(&input)

	if input.ObjectKey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	photoURL := "/storage/" + input.ObjectKey
	uID, _ := uuid.Parse(userID)

	err = h.db.Transaction(func(tx *gorm.DB) error {
		// Снимаем статус "текущая" со всех старых аватарок юзера
		tx.Model(&models.ProfileAvatar{}).Where("user_id = ?", uID).Update("is_current", false)

		// Создаем новую запись в истории
		newAvatar := models.ProfileAvatar{
			ID:        uuid.New(),
			UserID:    uID,
			URL:       photoURL,
			IsCurrent: true,
		}
		if err := tx.Create(&newAvatar).Error; err != nil {
			return err
		}

		// Обновляем основную ссылку в таблице profiles (для быстрого доступа в чате)
		return tx.Model(&models.Profile{}).Where("user_id = ?", uID).Update("photo_url", photoURL).Error
	})
	// Уведомляем систему через Kafka
	// h.KafkaWriter.WriteMessages(...)

	return c.JSON(http.StatusOK, map[string]string{"photo_url": photoURL})
}

// func (h *Handler) UploadPhotoToS3(c echo.Context) error {
// 	userVal := c.Get("user")
// 	userID, err := getSenderID(c, h, userVal)
// 	if err != nil {
// 		return err
// 	}

// 	// Получаем файл из запроса
// 	file, _ := c.FormFile("photo")
// 	src, _ := file.Open()
// 	defer src.Close()

// 	// Генерируем уникальное имя (объект)
// 	objectName := uuid.New().String() + ".jpg"

// 	// Загружаем в Minio (S3)
// 	ctx := c.Request().Context()

// 	_, err = h.S3Client.PutObject(ctx, "profiles-bucket", objectName, src, file.Size, minio.PutObjectOptions{
// 		ContentType: "image/jpeg",
// 	})

// 	// 4. Генерируем публичную ссылку
// 	photoURL := fmt.Sprintf("http://localhost:9000/profiles-bucket/%s", objectName)

// 	// 5. Сохраняем в Postgres
// 	h.DB.Model(&models.Profile{}).Where("user_id = ?", userID).Update("photo_url", photoURL)

// 	return c.JSON(200, map[string]string{"url": photoURL})
// }
