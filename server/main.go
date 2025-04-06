package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем все подключения
	},
}

// VideoWriter управляет сохранением потока H.264 в файл
type VideoWriter struct {
	mutex      sync.Mutex
	outputFile *os.File
	filePath   string
}

// NewVideoWriter создает новый экземпляр VideoWriter
func NewVideoWriter(outputDir string) (*VideoWriter, error) {
	// Создаем директорию, если она не существует
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию: %v", err)
	}

	// Генерируем имя файла на основе текущего времени
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filePath := filepath.Join(outputDir, fmt.Sprintf("webcam_%s.h264", timestamp))

	// Создаем файл для записи
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать файл: %v", err)
	}

	log.Printf("Запись в файл: %s", filePath)

	return &VideoWriter{
		outputFile: file,
		filePath:   filePath,
	}, nil
}

// Write записывает данные в файл
func (vw *VideoWriter) Write(data []byte) error {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()

	_, err := vw.outputFile.Write(data)
	return err
}

// Close закрывает файл
func (vw *VideoWriter) Close() error {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()

	if vw.outputFile != nil {
		log.Printf("Закрытие файла: %s", vw.filePath)
		err := vw.outputFile.Close()
		vw.outputFile = nil
		return err
	}
	return nil
}

func main() {
	// Парсинг флагов командной строки
	port := flag.Int("port", 8080, "порт для запуска сервера")
	outputDir := flag.String("output", "recordings", "директория для сохранения записей")
	flag.Parse()

	// Обработчик WebSocket подключений
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Ошибка при апгрейде до WebSocket: %v", err)
			return
		}
		defer conn.Close()

		// Создаем файл для сохранения потока
		videoWriter, err := NewVideoWriter(*outputDir)
		if err != nil {
			log.Printf("Не удалось создать запись: %v", err)
			return
		}
		defer videoWriter.Close()

		clientAddr := conn.RemoteAddr().String()
		log.Printf("Клиент подключен: %s", clientAddr)

		// Обработка входящих сообщений
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Ошибка чтения: %v", err)
				break
			}

			// Обрабатываем только бинарные сообщения (закодированные видеоданные)
			if messageType == websocket.BinaryMessage {
				err = videoWriter.Write(message)
				if err != nil {
					log.Printf("Ошибка записи данных: %v", err)
					break
				}
			}
		}

		log.Printf("Клиент отключен: %s", clientAddr)
	})

	// Создаем простую страницу-статус
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Сервер стриминга веб-камеры</title>
			<style>
				body { font-family: Arial, sans-serif; margin: 40px; }
				.status { padding: 20px; background-color: #e0f7fa; border-radius: 5px; }
			</style>
		</head>
		<body>
			<h1>Сервер стриминга веб-камеры</h1>
			<div class="status">
				<p>✅ Сервер запущен и принимает соединения</p>
				<p>Директория для записей: <code>` + *outputDir + `</code></p>
			</div>
		</body>
		</html>
		`))
	})

	// Запускаем HTTP-сервер
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Запуск сервера на порту %d...", *port)
	log.Printf("Статус сервера доступен по адресу http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
