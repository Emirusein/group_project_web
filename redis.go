package auth

import (
	"crypto/rand"
	"encoding/hex"

	//"fmt"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

var rdb *redis.Client // Основной клиент Redis

// Инициализация Redis
func InitRedis() error {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Проверяем соединение
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	log.Println("Redis успешно инициализирован")
	rdb = client // Присваиваем глобальной переменной
	return nil
}

// Функция для хранения данных в Redis
func SetCache(key string, value string) error {
	err := rdb.Set(context.Background(), key, value, 0).Err()
	if err != nil {
		log.Printf("Error setting cache: %v", err)
		return err
	}
	return nil
}

// Установка статуса пользователя
func SetUserStatus(sessionToken string, status string) error {
	ctx := context.Background()
	return rdb.HSet(ctx, sessionToken, "status", status).Err()
}

// Получение статуса пользователя
func GetUserStatus(sessionToken string) (string, error) {
	ctx := context.Background()
	return rdb.HGet(ctx, sessionToken, "status").Result()
}

// Удаление статуса пользователя
func DeleteUserStatus(sessionToken string) error {
	ctx := context.Background()
	return rdb.Del(ctx, sessionToken).Err()
}

func GenerateSessionToken() string {
	bytes := make([]byte, 16) // 16 байт для токена (128 бит)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Error generating session token: %v", err)
		return ""
	}
	return hex.EncodeToString(bytes)
}
