package camera

import (
	"io"

	"github.com/pion/mediadevices"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // Регистрируем драйвер камеры
	"github.com/pion/mediadevices/pkg/prop"

	"webcam-transfer/client/internal/application"
	"webcam-transfer/client/internal/domain"
)

// MediaDevicesManager реализация CameraManager с использованием библиотеки mediadevices
type MediaDevicesManager struct {
	logger application.Logger
}

// NewMediaDevicesManager создает новый менеджер медиаустройств
func NewMediaDevicesManager(logger application.Logger) *MediaDevicesManager {
	return &MediaDevicesManager{
		logger: logger,
	}
}

// ListDevices возвращает список доступных устройств захвата
func (m *MediaDevicesManager) ListDevices() ([]domain.VideoDevice, error) {
	devices := mediadevices.EnumerateDevices()
	result := make([]domain.VideoDevice, 0, len(devices))

	for _, device := range devices {
		result = append(result, domain.VideoDevice{
			ID:    device.DeviceID,
			Label: device.Label,
			Kind:  string(device.Kind),
		})
	}

	return result, nil
}

// OpenCamera открывает камеру с заданными параметрами
func (m *MediaDevicesManager) OpenCamera(config domain.VideoConfig) (domain.VideoTrack, error) {
	// Настройка кодека H.264
	// Вместо прямого использования x264, используем опцию VP8 из базового пакета
	// Это позволит нам снизить зависимость от внешних библиотек

	// Настройка захвата
	constraints := mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			// Задаем предпочтительные параметры, но не строгие
			c.Width = prop.Int(int32(config.Width))
			c.Height = prop.Int(int32(config.Height))

			// Если указан конкретный ID устройства
			if config.DeviceID != "" {
				c.DeviceID = prop.String(config.DeviceID)
			}
		},
	}

	// Пробуем получить медиа поток
	mediaStream, err := mediadevices.GetUserMedia(constraints)
	if err != nil {
		m.logger.Error("Ошибка с исходными ограничениями: %v", err)

		// Пробуем с еще более простыми ограничениями
		m.logger.Info("Пробуем с минимальными ограничениями...")
		constraints = mediadevices.MediaStreamConstraints{
			Video: func(c *mediadevices.MediaTrackConstraints) {
				// Никаких форматных ограничений
				if config.DeviceID != "" {
					c.DeviceID = prop.String(config.DeviceID)
				}
			},
		}

		mediaStream, err = mediadevices.GetUserMedia(constraints)
		if err != nil {
			m.logger.Error("Не удалось получить доступ к медиа-устройству: %v", err)
			return nil, err
		}
	}

	// Получаем видеотреки
	videoTracks := mediaStream.GetVideoTracks()
	if len(videoTracks) == 0 {
		m.logger.Error("Видеотрек не обнаружен")
		return nil, err
	}

	return &MediaDevicesTrack{
		track:  videoTracks[0],
		logger: m.logger,
	}, nil
}

// MediaDevicesTrack обертка для MediaDevices Track
type MediaDevicesTrack struct {
	track  mediadevices.Track
	logger application.Logger
}

// ID возвращает идентификатор трека
func (t *MediaDevicesTrack) ID() string {
	return t.track.ID()
}

// Close закрывает трек
func (t *MediaDevicesTrack) Close() error {
	return t.track.Close()
}

// CreateReader создает ридер для чтения видеокадров
func (t *MediaDevicesTrack) CreateReader() (domain.VideoReader, error) {
	// Используем простой кодек, который точно доступен
	reader, err := t.track.NewEncodedIOReader("vp8")
	if err != nil {
		// Если VP8 недоступен, попробуем работать с необработанными данными
		t.logger.Info("VP8 кодек недоступен, пробуем H264...")
		reader, err = t.track.NewEncodedIOReader("h264")

		if err != nil {
			t.logger.Error("Ошибка создания ридера: %v", err)
			return nil, err
		}
	}

	return &MediaDevicesReader{
		reader:      reader,
		logger:      t.logger,
		frameNumber: 0,
	}, nil
}

// MediaDevicesReader обертка для MediaDevices Reader
type MediaDevicesReader struct {
	reader      io.ReadCloser
	logger      application.Logger
	frameNumber int
	buffer      []byte
}

// Read читает следующий кадр
func (r *MediaDevicesReader) Read() (*domain.VideoFrame, error) {
	if r.buffer == nil {
		r.buffer = make([]byte, 1024*1024) // 1MB буфер для кадра
	}

	n, err := r.reader.Read(r.buffer)
	if err != nil {
		if err != io.EOF {
			r.logger.Error("Ошибка чтения данных: %v", err)
		}
		return nil, err
	}

	if n > 0 {
		r.frameNumber++
		// Копируем данные, чтобы избежать проблем с перезаписью буфера
		data := make([]byte, n)
		copy(data, r.buffer[:n])

		return &domain.VideoFrame{
			Data:   data,
			Size:   n,
			Number: r.frameNumber,
		}, nil
	}

	return nil, nil
}

// Close закрывает ридер
func (r *MediaDevicesReader) Close() error {
	return r.reader.Close()
}
