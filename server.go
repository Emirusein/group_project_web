package main

import (
	//"bytes"
	"database/sql"
	"encoding/json"
	"go-auth/auth"
	"go-auth/database"
	"go-auth/handlers"
	"log"
	"net/http"
	"os"

	//"os/exec"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// Хранилище тестов в памяти (для упрощения)
var testStorage []struct {
	Name      string   `json:"name"`
	Questions []string `json:"questions"`
}
var testMutex sync.Mutex // Для потокобезопасности

type Test struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	Creator   string     `json:"creator"`
	Questions []Question `json:"questions"`
}

type Question struct {
	QuestionText string   `json:"question_text"`
	Options      []string `json:"options"`
}

func main() {
	// Загрузка переменных из .env файла
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Ошибка загрузки .env файла: %v", err)
	}

	// Загружаем секретный ключ
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		log.Fatal("JWT_SECRET_KEY не установлен")
	}
	log.Println("Loaded JWT_SECRET_KEY:", secretKey)

	// Передаем секретный ключ в auth.SecretKey
	auth.SecretKey = secretKey

	// Инициализация MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	err = database.InitMongoDB(mongoURI)
	if err != nil {
		log.Printf("Ошибка подключения к MongoDB: %v. Приложение продолжит работу без подключения к базе данных.", err)
	}

	// Инициализация Redis
	err = auth.InitRedis()
	if err != nil {
		log.Printf("Ошибка подключения к Redis: %v. Приложение продолжит работу без подключения к Redis.", err)
	}

	// Инициализация Gin
	r := gin.Default()

	// Загружаем шаблоны из папки templates
	r.LoadHTMLGlob("../templates/*")

	// Добавляем маршруты
	r.GET("/", handlers.IndexHandler)
	r.GET("/auth/yandex", handlers.YandexAuthHandler)
	r.GET("/auth/github", handlers.GithubAuthHandler)
	r.GET("/login", handlers.LoginHandler)
	r.GET("/yandex/login", handlers.YandexLoginHandler)
	r.GET("/yandex/callback", handlers.YandexCallbackHandler)
	r.GET("/github/login", handlers.GitHubLoginHandler)
	r.GET("/github/callback", handlers.GitHubCallbackHandler)
	r.POST("/verify_code", handlers.VerifyCodeHandler)
	r.GET("/request_code", handlers.RequestCodeForm)
	r.POST("/request_code", handlers.RequestCodeHandler)
	r.GET("/protected", handlers.ProtectedHandler)

	//ИНТЕГРАЦИЯ ГЛАВНОГО МОДУЛЯ
	r.GET("/profile", handlers.ProfileHandler) // Восстановленный маршрут

	r.Static("/static", "./static") // Подгрузка static с JavaScript, Css, e.t.c

	http.Handle("/styles/", http.StripPrefix("/styles/", http.FileServer(http.Dir("./styles")))) //подключение стилей в /styles

	// Указание пути к базе данных
	dbPath := "./tests.db"

	// Обработчик для получения списка тестов
	r.POST("/api/tests", func(c *gin.Context) {
		var test Test // Инициализируем переменную для хранения данных теста

		// Логирование входящего запроса
		log.Printf("Получен запрос на добавление теста: %v", c.Request.Body)

		// Привязка полученного JSON к структуре Test
		if err := c.ShouldBindJSON(&test); err != nil {
			log.Printf("Ошибка привязки JSON: %v", err)                      // Логируем ошибку связывания
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"}) // Возвращаем ошибку 400
			return
		}

		// Логируем данные, которые были привязаны из запроса
		log.Printf("Привязанный тест: %+v", test)

		// Сериализация полей вопросов теста
		questionsJSON, err := json.Marshal(test.Questions)
		if err != nil {
			log.Printf("Ошибка сериализации вопросов: %v", err)                                     // Логируем ошибку сериализации
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize questions"}) // Возвращаем ошибку 500
			return
		}

		// Логируем сериализованные вопросы для проверки
		log.Printf("Сериализованные вопросы: %s", string(questionsJSON))

		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath) // Открываем соединение с БД
		if err != nil {
			log.Printf("Ошибка подключения к базе данных: %v", err)                                 // Логируем ошибку подключения
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"}) // Возвращаем ошибку 500
			return
		}
		defer db.Close() // Обязательно закрываем соединение после завершения работы с ним

		// Выполняем SQL-запрос для добавления теста в базу данных
		_, err = db.Exec("INSERT INTO tests (name, questions, creator) VALUES (?, ?, ?)", test.Name, string(questionsJSON), "unknown_creator")
		if err != nil {
			log.Printf("Ошибка добавления теста в базу данных: %v", err)                  // Логируем ошибку при добавлении
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save test"}) // Возвращаем ошибку 500
			return
		}

		// Логируем успешное добавление теста
		log.Printf("Тест '%s' успешно добавлен в базу данных", test.Name)

		// Возвращаем успешный ответ клиенту
		c.JSON(http.StatusOK, gin.H{"message": "Test added successfully"}) // Возвращаем сообщение об успешном добавлении
	})

	r.GET("/create_test", func(c *gin.Context) {
		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			// Логируем ошибку подключения к БД
			log.Printf("Ошибка подключения к базе данных: %v", err)
			// Возвращаем страницу с сообщением об ошибке
			c.HTML(http.StatusInternalServerError, "create_test.html", gin.H{"error": "Ошибка подключения к базе данных"})
			return
		}
		defer db.Close() // Обязательно закрываем соединение после завершения работы с ним

		// Выполняем SQL-запрос для получения всех тестов
		rows, err := db.Query("SELECT id, name, questions, creator FROM tests")
		if err != nil {
			// Логируем ошибку выполнения SQL-запроса
			log.Printf("Ошибка выполнения SQL-запроса: %v", err)
			// Возвращаем страницу с сообщением об ошибке
			c.HTML(http.StatusInternalServerError, "create_test.html", gin.H{"error": "Ошибка загрузки тестов"})
			return
		}
		defer rows.Close() // Закрываем rows после завершения работы

		// Инициализируем срез для хранения тестов
		var tests []map[string]interface{}

		// Проходим по всем строкам результата запроса
		for rows.Next() {
			var id int
			var name, questionsSerialized, creator string
			// Сканируем данные из строки
			if err := rows.Scan(&id, &name, &questionsSerialized, &creator); err != nil {
				// Логируем ошибку при сканировании данных
				log.Printf("Ошибка сканирования данных: %v", err)
				// Возвращаем страницу с сообщением об ошибке
				c.HTML(http.StatusInternalServerError, "create_test.html", gin.H{"error": "Ошибка обработки тестов"})
				return
			}

			// Десериализация строковых данных вопросов в массив
			questions := strings.Split(strings.TrimRight(questionsSerialized, ";"), ";")

			// Сохраняем тест в коллекцию
			tests = append(tests, gin.H{
				"id":        id,
				"name":      name,
				"questions": questions,
				"creator":   creator,
			})
		}

		// Возвращаем HTML-шаблон с загруженными тестами
		c.HTML(http.StatusOK, "create_test.html", gin.H{"tests": tests})
	})

	// Удаление тестов
	// Обработчик для удаления теста по ID
	r.DELETE("/api/tests/:id", func(c *gin.Context) {
		// Получаем ID теста из параметров запроса
		testID := c.Param("id")

		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			// Логируем ошибку подключения к БД
			log.Printf("Ошибка подключения к базе данных: %v", err)
			// Возвращаем ошибку 500 с соответствующим сообщением
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to database"})
			return
		}
		defer db.Close() // Обязательно закрываем соединение после завершения работы с ним

		// Выполняем SQL-запрос для удаления теста по ID
		result, err := db.Exec("DELETE FROM tests WHERE id = ?", testID)
		if err != nil {
			// Логируем ошибку при удалении теста
			log.Printf("Ошибка удаления теста: %v", err)
			// Возвращаем ошибку 500 с соответствующим сообщением
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete test"})
			return
		}

		// Проверяем, был ли удален хотя бы один тест
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			// Если нет удаленных строк, тест не найден
			log.Printf("Тест с ID %s не найден", testID)                    // Логируем информацию о том, что тест не найден
			c.JSON(http.StatusBadRequest, gin.H{"error": "Test not found"}) // Возвращаем ошибку 400
			return
		}

		// Если удаление прошло успешно, отправляем ответ
		c.JSON(http.StatusOK, gin.H{"message": "Test deleted successfully"}) // Возвращаем сообщение об успешном удалении
	})

	// Обработчик для получения всех тестов
	r.GET("/tests", func(c *gin.Context) {
		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			// Логируем ошибку подключения к БД
			log.Printf("Ошибка подключения к базе данных: %v", err)
			// Возвращаем страницу с сообщением об ошибке
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Ошибка подключения к базе данных"})
			return
		}
		defer db.Close() // Обязательно закрываем соединение после завершения работы с ним

		// Выполняем SQL-запрос для получения всех тестов
		rows, err := db.Query("SELECT id, name, questions, creator FROM tests")
		if err != nil {
			// Логируем ошибку выполнения SQL-запроса
			log.Printf("Ошибка выполнения SQL-запроса: %v", err)
			// Возвращаем страницу с сообщением об ошибке
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Ошибка загрузки тестов"})
			return
		}
		defer rows.Close() // Обязательно закрываем rows после завершения работы с ними

		// Инициализируем структуру для хранения полученных тестов
		var tests []Test
		for rows.Next() {
			// Создаем временную структуру для хранения теста
			var test Test
			var questionsSerialized string

			// Сканируем строки
			if err := rows.Scan(&test.ID, &test.Name, &questionsSerialized, &test.Creator); err != nil {
				// Логируем ошибку при сканировании данных
				log.Printf("Ошибка сканирования данных: %v", err)
				// Возвращаем страницу с сообщением об ошибке
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Ошибка обработки тестов"})
				return
			}

			// Десериализуем поле `questions` из JSON
			if err := json.Unmarshal([]byte(questionsSerialized), &test.Questions); err != nil {
				// Логируем ошибку при десериализации вопросов
				log.Printf("Ошибка десериализации вопросов: %v", err)
				// Возвращаем страницу с сообщением об ошибке
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Ошибка обработки вопросов"})
				return
			}

			// Добавляем тест в срез
			tests = append(tests, test)
		}

		// Передаём полученные тесты в HTML-шаблон для отображения
		c.HTML(http.StatusOK, "tests.html", gin.H{"tests": tests})
	})

	// Обработчик для получения теста по ID
	r.GET("/api/tests/:id", func(c *gin.Context) {
		// Извлекаем ID теста из параметров запроса
		id := c.Param("id")

		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			// Возвращаем ошибку 500 в случае неудачного подключения к БД
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подключения к базе данных"})
			return
		}
		defer db.Close() // Обязательно закрываем соединение после использования

		// Получаем тест по ID с помощью вспомогательной функции
		test, err := getTestByID(db, id)
		if err != nil {
			// Возвращаем ошибку 500, если возникла проблема при получении теста
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения теста"})
			return
		}

		// Возвращаем тест в формате JSON с кодом 200
		c.JSON(http.StatusOK, test)
	})

	// Обработчик для отправки ответов на тест по ID
	r.POST("/api/tests/:id/submit", func(c *gin.Context) {
		// Извлекаем ID теста из параметров запроса
		id := c.Param("id")

		// Структура для хранения данных отправки от пользователя
		var submission struct {
			UserID  int               `json:"user_id"` // ID пользователя
			Answers map[string]string `json:"answers"` // Ответы на вопросы в формате "вопрос -> ответ"
		}

		// Пробуем связать входящие данные с созданной структурой
		if err := c.ShouldBindJSON(&submission); err != nil {
			// Если данные некорректны, возвращаем ошибку 400 (Bad Request)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
			return
		}

		// Подключаемся к базе данных
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			// Логируем ошибку подключения к БД
			log.Printf("Ошибка подключения к базе данных: %v", err)
			// Возвращаем ошибку 500 (Internal Server Error)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка подключения к базе данных"})
			return
		}
		defer db.Close() // Обязательно закрываем соединение после завершения работы с ним

		// Сохраняем ответы в базе данных
		for question, answer := range submission.Answers {
			// Выполняем SQL-запрос для вставки ответа пользователя
			_, err := db.Exec("INSERT INTO answers (test_id, user_id, question, answer) VALUES (?, ?, ?, ?)",
				id, submission.UserID, question, answer)
			if err != nil {
				// Если возникает ошибка при сохранении, логируем её и возвращаем ошибку 500
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения ответа"})
				return
			}
		}

		// Отправляем успешный ответ обратно с сообщением
		c.JSON(http.StatusOK, gin.H{"message": "Ответы успешно сохранены"})
	})

	log.Println("REDIS_ADDR:", os.Getenv("REDIS_ADDR"))
	r.Run(":8080")
}

// Функция для получения теста по ID
func getTestByID(db *sql.DB, id string) (Test, error) {
	var test Test
	var testID int
	if err := json.Unmarshal([]byte(id), &testID); err != nil {
		return test, err
	}

	row := db.QueryRow("SELECT id, name, creator FROM tests WHERE id = ?", testID)
	if err := row.Scan(&test.ID, &test.Name, &test.Creator); err != nil {
		return test, err
	}

	// Получение вопросов для теста
	questions, err := getQuestionsByTestID(db, test.ID)
	if err != nil {
		return test, err
	}
	test.Questions = questions

	return test, nil
}

// Функция для получения вопросов теста по ID
func getQuestionsByTestID(db *sql.DB, testID int) ([]Question, error) {
	var questionsSerialized string
	err := db.QueryRow("SELECT questions FROM tests WHERE id = ?", testID).Scan(&questionsSerialized)
	if err != nil {
		return nil, err
	}

	// Десериализация вопросов из JSON
	var questions []Question
	if err := json.Unmarshal([]byte(questionsSerialized), &questions); err != nil {
		return nil, err
	}

	return questions, nil
}
