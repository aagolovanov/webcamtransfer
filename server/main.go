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

// VideoWriter управляет сохранением потока H.264 и трансляцией
type VideoWriter struct {
	mutex        sync.Mutex
	outputFile   *os.File
	filePath     string
	broadcasters map[*websocket.Conn]bool
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
		outputFile:   file,
		filePath:     filePath,
		broadcasters: make(map[*websocket.Conn]bool),
	}, nil
}

// Write записывает данные в файл и рассылает всем подключенным клиентам
func (vw *VideoWriter) Write(data []byte) error {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()

	// Записываем в файл
	if vw.outputFile != nil {
		_, err := vw.outputFile.Write(data)
		if err != nil {
			return err
		}
	}

	// Рассылаем всем подключенным клиентам
	for conn := range vw.broadcasters {
		err := conn.WriteMessage(websocket.BinaryMessage, data)
		if err != nil {
			log.Printf("Ошибка трансляции: %v", err)
			conn.Close()
			delete(vw.broadcasters, conn)
		}
	}

	return nil
}

// AddBroadcaster добавляет нового подписчика на трансляцию
func (vw *VideoWriter) AddBroadcaster(conn *websocket.Conn) {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()
	vw.broadcasters[conn] = true
}

// RemoveBroadcaster удаляет подписчика
func (vw *VideoWriter) RemoveBroadcaster(conn *websocket.Conn) {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()
	delete(vw.broadcasters, conn)
}

// Close закрывает файл и все соединения
func (vw *VideoWriter) Close() error {
	vw.mutex.Lock()
	defer vw.mutex.Unlock()

	// Закрываем все соединения трансляции
	for conn := range vw.broadcasters {
		conn.Close()
		delete(vw.broadcasters, conn)
	}

	// Закрываем файл
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

	// Создаем глобальный VideoWriter для трансляции
	videoWriter, err := NewVideoWriter(*outputDir)
	if err != nil {
		log.Fatalf("Не удалось создать VideoWriter: %v", err)
	}
	defer videoWriter.Close()

	// Обработчик WebSocket подключений от веб-камеры (отправка данных)
	http.HandleFunc("/ws/sender", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Ошибка при апгрейде до WebSocket: %v", err)
			return
		}
		defer conn.Close()

		clientAddr := conn.RemoteAddr().String()
		log.Printf("Клиент (отправитель) подключен: %s", clientAddr)

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

		log.Printf("Клиент (отправитель) отключен: %s", clientAddr)
	})

	// Обработчик WebSocket подключений для просмотра трансляции
	http.HandleFunc("/ws/viewer", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Ошибка при апгрейде до WebSocket: %v", err)
			return
		}
		defer conn.Close()

		// Добавляем нового подписчика
		videoWriter.AddBroadcaster(conn)
		defer videoWriter.RemoveBroadcaster(conn)

		clientAddr := conn.RemoteAddr().String()
		log.Printf("Клиент (зритель) подключен: %s", clientAddr)

		// Ждем, пока клиент не отключится
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}

		log.Printf("Клиент (зритель) отключен: %s", clientAddr)
	})

	// Создаем страницу для просмотра трансляции
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title></title>
			<style>
				body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f0f0f0; }
				.container { max-width: 800px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
				h1 { color: #333; text-align: center; }
				#videoContainer { width: 100%; margin: 20px 0; }
				video { width: 100%; background: black; }
			</style>
		</head>
		<body>
			<div class="container">
				<h1>AAAAAAAAAAAA</h1>
				<div id="videoContainer">
					<video id="videoPlayer" controls autoplay></video>
				</div>
			</div>

			<script>
				const videoPlayer = document.getElementById('videoPlayer');
				const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
				const wsUrl = wsProtocol + '//' + window.location.host + '/ws/viewer';
				const ws = new WebSocket(wsUrl);

				// Используем MSE (Media Source Extensions) для воспроизведения H.264
				const mimeCodec = 'video/mp4; codecs="avc1.42E01E"';
				
				if ('MediaSource' in window && MediaSource.isTypeSupported(mimeCodec)) {
					const mediaSource = new MediaSource();
					videoPlayer.src = URL.createObjectURL(mediaSource);
					
					mediaSource.addEventListener('sourceopen', () => {
						const sourceBuffer = mediaSource.addSourceBuffer(mimeCodec);
						
						ws.onmessage = function(event) {
							if (event.data instanceof Blob) {
								const reader = new FileReader();
								reader.onload = function() {
									if (sourceBuffer.updating === false && mediaSource.readyState === 'open') {
										sourceBuffer.appendBuffer(new Uint8Array(reader.result));
									}
								};
								reader.readAsArrayBuffer(event.data);
							}
						};
					});
				} else {
					console.error('Unsupported MIME type or codec: ', mimeCodec);
					videoPlayer.innerHTML = 'Ваш браузер не поддерживает воспроизведение этого видео.';
				}

				ws.onclose = function() {
					console.log('WebSocket соединение закрыто');
				};

				ws.onerror = function(error) {
					console.error('WebSocket ошибка:', error);
				};
			</script>
		</body>
		</html>
		`))
	})

	// Главная страница с информацией
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Server of striming</title>
			<style>
				body { font-family: Arial, sans-serif; margin: 40px; }
				.status { padding: 20px; background-color: #e0f7fa; border-radius: 5px; }
				a { color: #0066cc; text-decoration: none; }
				a:hover { text-decoration: underline; }
			</style>
		</head>
		<body>
			<h1>Server of striming</h1>
			<div class="status">
				<p>Server is ready</p>
				<p>Recordings: <code>` + *outputDir + `</code></p>
				<p><a href="/stream">Stream</a></p>
			</div>
		</body>
		</html>
		`))
	})

	// Запускаем HTTP-сервер
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Запуск сервера на порту %d...", *port)
	log.Printf("Статус сервера доступен по адресу http://localhost%s", addr)
	log.Printf("Трансляция доступна по адресу http://localhost%s/stream", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
