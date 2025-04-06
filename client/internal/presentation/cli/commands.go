package cli

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"webcam-transfer/client/internal/application"
	"webcam-transfer/client/internal/domain"
)

// CLI представляет CLI интерфейс приложения
type CLI struct {
	webcamService *application.WebcamService
	logger        application.Logger
	config        *Config
}

// Config представляет конфигурацию CLI
type Config struct {
	Address     string
	Width       int
	Height      int
	FPS         int
	BitRate     int
	Debug       bool
	ListDevices bool
	DeviceID    string
}

// NewCLI создает новый CLI интерфейс
func NewCLI(webcamService *application.WebcamService, logger application.Logger) *CLI {
	return &CLI{
		webcamService: webcamService,
		logger:        logger,
	}
}

// SetConfig устанавливает конфигурацию напрямую
func (c *CLI) SetConfig(config *Config) {
	c.config = config
}

// ParseFlags парсит аргументы командной строки
func (c *CLI) ParseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Address, "addr", "localhost:8080", "адрес сервера")
	flag.IntVar(&config.Width, "width", 640, "ширина видео")
	flag.IntVar(&config.Height, "height", 480, "высота видео")
	flag.IntVar(&config.FPS, "fps", 30, "частота кадров")
	flag.IntVar(&config.BitRate, "bitrate", 1_000_000, "битрейт видео (bps)")
	flag.BoolVar(&config.Debug, "debug", false, "включить отладочные сообщения")
	flag.BoolVar(&config.ListDevices, "list-devices", false, "показать список доступных камер и выйти")
	flag.StringVar(&config.DeviceID, "device", "", "ID устройства камеры для использования")

	flag.Parse()

	c.config = config
	return config
}

// Run запускает CLI
func (c *CLI) Run() error {
	// Если нужно вывести список устройств
	if c.config.ListDevices {
		return c.listDevices()
	}

	// Настраиваем обработку сигналов завершения
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	// Обработка сигналов

	// Создаем конфигурацию для видеопотока
	videoConfig := domain.VideoConfig{
		Width:        c.config.Width,
		Height:       c.config.Height,
		FrameRate:    c.config.FPS,
		BitRate:      c.config.BitRate,
		DeviceID:     c.config.DeviceID,
		CodecName:    "h264",
		StreamingURL: fmt.Sprintf("ws://%s/ws", c.config.Address),
	}

	// Запускаем захват видео
	err := c.webcamService.StartCapture(videoConfig)
	if err != nil {
		return err
	}

	// Ожидаем сигнала завершения
	<-interrupt
	c.logger.Info("Прерывание получено, закрытие...")

	// Останавливаем захват
	return c.webcamService.StopCapture()
}

// listDevices выводит список доступных устройств
func (c *CLI) listDevices() error {
	devices, err := c.webcamService.ListDevices()
	if err != nil {
		return err
	}

	fmt.Println("Доступные устройства:")
	for i, device := range devices {
		fmt.Printf("[%d] %s (%s)\n", i, device.Label, device.Kind)
	}

	return nil
}
