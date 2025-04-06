package application

import (
	"context"
	"errors"
	"sync"

	"webcam-transfer/client/internal/domain"
)

// WebcamService сервис для работы с веб-камерой и стримингом
type WebcamService struct {
	cameraManager CameraManager
	streamManager StreamManager
	logger        Logger

	activeTrack   domain.VideoTrack
	streamContext context.Context
	cancelFunc    context.CancelFunc
	mutex         sync.Mutex
}

// NewWebcamService создает новый сервис для работы с веб-камерой
func NewWebcamService(cameraManager CameraManager, streamManager StreamManager, logger Logger) *WebcamService {
	return &WebcamService{
		cameraManager: cameraManager,
		streamManager: streamManager,
		logger:        logger,
	}
}

// ListDevices возвращает список доступных устройств захвата
func (s *WebcamService) ListDevices() ([]domain.VideoDevice, error) {
	devices, err := s.cameraManager.ListDevices()
	if err != nil {
		s.logger.Error("Ошибка получения списка устройств: %v", err)
		return nil, err
	}
	return devices, nil
}

// StartCapture начинает захват и стриминг с указанной конфигурацией
func (s *WebcamService) StartCapture(config domain.VideoConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Если есть активный стрим, останавливаем его
	if s.activeTrack != nil {
		s.StopCapture()
	}

	// Открываем камеру
	s.logger.Info("Открытие камеры с параметрами: %dx%d, %d fps, битрейт: %d bps",
		config.Width, config.Height, config.FrameRate, config.BitRate)

	track, err := s.cameraManager.OpenCamera(config)
	if err != nil {
		s.logger.Error("Ошибка открытия камеры: %v", err)
		return err
	}

	s.activeTrack = track
	s.logger.Info("Используется камера: %s", track.ID())

	// Начинаем стриминг
	s.streamContext, s.cancelFunc = context.WithCancel(context.Background())

	go func() {
		err := s.streamManager.StartStreaming(s.streamContext, track, config)
		if err != nil {
			s.logger.Error("Ошибка стриминга: %v", err)
		}
	}()

	return nil
}

// StopCapture останавливает захват и стриминг
func (s *WebcamService) StopCapture() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.activeTrack == nil {
		return errors.New("нет активного захвата")
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}

	err := s.streamManager.StopStreaming()
	if err != nil {
		s.logger.Error("Ошибка остановки стриминга: %v", err)
	}

	err = s.activeTrack.Close()
	if err != nil {
		s.logger.Error("Ошибка закрытия трека: %v", err)
	}

	s.activeTrack = nil
	s.cancelFunc = nil

	return nil
}
