package streaming

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"webcam-transfer/client/internal/application"
	"webcam-transfer/client/internal/domain"
)

// WebSocketStreamer реализует стриминг видео через WebSocket
type WebSocketStreamer struct {
	conn         *websocket.Conn
	logger       application.Logger
	connected    bool
	mutex        sync.Mutex
	frameCounter int
	startTime    time.Time
	debugMode    bool
}

// NewWebSocketStreamer создает новый WebSocket стример
func NewWebSocketStreamer(logger application.Logger, debugMode bool) *WebSocketStreamer {
	return &WebSocketStreamer{
		logger:    logger,
		debugMode: debugMode,
	}
}

// StartStreaming начинает стриминг видео
func (s *WebSocketStreamer) StartStreaming(ctx context.Context, track domain.VideoTrack, config domain.VideoConfig) error {
	s.mutex.Lock()

	// Если уже подключены, отключаемся сначала
	if s.connected {
		s.mutex.Unlock()
		s.StopStreaming()
		s.mutex.Lock()
	}

	// Подключаемся к серверу
	u, err := url.Parse(config.StreamingURL)
	if err != nil {
		s.logger.Error("Некорректный URL стриминга: %v", err)
		s.mutex.Unlock()
		return err
	}

	s.logger.Info("Подключение к %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		s.logger.Error("Ошибка подключения к серверу: %v", err)
		s.mutex.Unlock()
		return err
	}

	s.conn = conn
	s.connected = true
	s.frameCounter = 0
	s.startTime = time.Now()
	s.mutex.Unlock()

	s.logger.Info("Подключено к серверу")

	// Создаем ридер для чтения видеокадров
	reader, err := track.CreateReader()
	if err != nil {
		s.logger.Error("Ошибка создания ридера: %v", err)
		s.StopStreaming()
		return err
	}
	defer reader.Close()

	s.logger.Info("Начало стриминга видео...")

	// Стриминг кадров
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Стриминг остановлен")
			return nil
		default:
			// Читаем следующий кадр
			frame, err := reader.Read()
			if err != nil {
				s.logger.Error("Ошибка чтения кадра: %v", err)
				return err
			}

			if frame == nil {
				continue
			}

			// Отправляем кадр
			err = s.sendFrame(frame)
			if err != nil {
				s.logger.Error("Ошибка отправки кадра: %v", err)
				return err
			}
		}
	}
}

// StopStreaming останавливает стриминг
func (s *WebSocketStreamer) StopStreaming() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.connected || s.conn == nil {
		return nil
	}

	// Отправляем сообщение о закрытии
	err := s.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)

	if err != nil {
		s.logger.Error("Ошибка закрытия WebSocket: %v", err)
	}

	s.conn.Close()
	s.conn = nil
	s.connected = false

	return nil
}

// IsConnected возвращает статус подключения
func (s *WebSocketStreamer) IsConnected() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.connected
}

// SendFrame отправляет кадр через WebSocket
func (s *WebSocketStreamer) SendFrame(frame *domain.VideoFrame) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.connected || s.conn == nil {
		return nil
	}

	return s.conn.WriteMessage(websocket.BinaryMessage, frame.Data)
}

// sendFrame внутренний метод для отправки кадра
func (s *WebSocketStreamer) sendFrame(frame *domain.VideoFrame) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.connected || s.conn == nil {
		return nil
	}

	err := s.conn.WriteMessage(websocket.BinaryMessage, frame.Data)
	if err != nil {
		return err
	}

	s.frameCounter++

	// Отладочная информация
	if s.debugMode && s.frameCounter%30 == 0 {
		currentTime := time.Now()
		elapsed := currentTime.Sub(s.startTime).Seconds()
		fps := float64(s.frameCounter) / elapsed
		s.logger.Debug("Отправлено фреймов: %d, FPS: %.2f, Размер последнего фрейма: %d байт",
			s.frameCounter, fps, frame.Size)
	}

	return nil
}
