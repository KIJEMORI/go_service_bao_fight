package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// LoadEnvironment - загрузить из файла конфигурацию в environments
func LoadEnvironment() {
	err := godotenv.Load("configs/local.env")
	if err != nil {
		log.Fatal(map[string]string{"Error loading .env file": err.Error()})

	}
}

// getEnv - считать environment в формете string
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

// getEnvAsInt - считать environment в формете int
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}
