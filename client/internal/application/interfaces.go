package application

import (
	"context"

	"webcam-transfer/client/internal/domain"
)

// CameraManager интерфейс для управления камерой
type CameraManager interface {
	// ListDevices возвращает список доступных устройств захвата
	ListDevices() ([]domain.VideoDevice, error)

	// OpenCamera открывает камеру с заданными параметрами
	OpenCamera(config domain.VideoConfig) (domain.VideoTrack, error)
}

// StreamManager интерфейс для управления стримингом
type StreamManager interface {
	// StartStreaming начинает стриминг видео
	StartStreaming(ctx context.Context, track domain.VideoTrack, config domain.VideoConfig) error

	// StopStreaming останавливает стриминг
	StopStreaming() error
}

// Logger интерфейс для логирования
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}
