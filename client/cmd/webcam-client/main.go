package main

import (
	"log"

	"webcam-transfer/client/internal/application"
	"webcam-transfer/client/internal/infrastructure/camera"
	"webcam-transfer/client/internal/infrastructure/logger"
	"webcam-transfer/client/internal/infrastructure/streaming"
	"webcam-transfer/client/internal/presentation/cli"
)

func main() {
	// Создаем CLI интерфейс для парсинга флагов 
	cliApp := cli.NewCLI(nil, nil)

	// Парсим флаги
	config := cliApp.ParseFlags()

	// Инициализируем логгер
	stdLogger := logger.NewStdLogger(config.Debug)

	// Инициализируем инфраструктурные компоненты
	cameraManager := camera.NewMediaDevicesManager(stdLogger)
	streamManager := streaming.NewWebSocketStreamer(stdLogger, config.Debug)

	// Инициализируем сервис приложения
	webcamService := application.NewWebcamService(cameraManager, streamManager, stdLogger)

	// Внедряем сервис в CLI без повторного парсинга флагов
	cliApp = cli.NewCLI(webcamService, stdLogger)
	cliApp.SetConfig(config) // Устанавливаем конфигурацию напрямую без парсинга

	// Запускаем CLI
	err := cliApp.Run()
	if err != nil {
		log.Fatalf("Ошибка: %v", err)
	}
}
