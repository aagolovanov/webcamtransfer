package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/mediadevices"
	"github.com/pion/mediadevices/pkg/codec/x264"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // Регистрируем драйвер камеры
	"github.com/pion/mediadevices/pkg/prop"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "адрес сервера")
	width := flag.Int("width", 640, "ширина видео")
	height := flag.Int("height", 480, "высота видео")
	fps := flag.Int("fps", 30, "частота кадров")
	bitrate := flag.Int("bitrate", 1_000_000, "битрейт видео (bps)")
	debug := flag.Bool("debug", false, "включить отладочные сообщения")
	listDevices := flag.Bool("list-devices", false, "показать список доступных камер и выйти")
	deviceID := flag.String("device", "", "ID устройства камеры для использования")
	flag.Parse()

	// Вывод списка камер, если запрошено
	if *listDevices {
		devices := mediadevices.EnumerateDevices()
		fmt.Println("Доступные устройства:")
		for i, device := range devices {
			fmt.Printf("[%d] %s (%s)\n", i, device.Label, device.Kind)
		}
		return
	}

	// Настройка подключения к серверу
	u := url.URL{Scheme: "ws", Host: *addr, Path: "/ws"}
	log.Printf("Подключение к %s", u.String())

	// Настройка обработки сигналов завершения
	interrupt := make(chan os.Signal, 1)
	booted := make(chan struct{})
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		select {
		case <-booted:
			log.Println("Booted.")
			return
		case <-interrupt:
			log.Println("\nInterrupted on boot.")
			os.Exit(0)
		}
	}()

	// Настройка кодека H.264
	x264Params, err := x264.NewParams()
	if err != nil {
		log.Fatalf("Ошибка создания параметров x264: %v", err)
	}
	x264Params.BitRate = int(*bitrate)       // битрейт в bps
	x264Params.Preset = x264.PresetUltrafast // Использование самого быстрого пресета
	x264Params.KeyFrameInterval = 60         // Keyframe каждые 2 секунды при 30 fps

	log.Printf("Настройки видео: %dx%d, %d fps, битрейт: %d bps", *width, *height, *fps, *bitrate)

	// Выбор устройства захвата
	var mediaStream mediadevices.MediaStream

	// Настройка захвата с минимальными требованиями
	constraints := mediadevices.MediaStreamConstraints{
		Video: func(c *mediadevices.MediaTrackConstraints) {
			// Не устанавливаем жесткие ограничения на формат кадра
			// Позволить драйверу выбрать лучший формат

			// Задаем предпочтительные параметры, но не строгие
			c.Width = prop.Int(int32(*width))
			c.Height = prop.Int(int32(*height))

			// Если указан конкретный ID устройства
			if *deviceID != "" {
				c.DeviceID = prop.String(*deviceID)
			}
		},
		Codec: mediadevices.NewCodecSelector(
			mediadevices.WithVideoEncoders(&x264Params),
		),
	}

	// Пробуем получить медиа поток
	log.Println("Попытка доступа к видеоустройству...")
	mediaStream, err = mediadevices.GetUserMedia(constraints)
	if err != nil {
		log.Printf("Ошибка с исходными ограничениями: %v", err)

		// Пробуем с еще более простыми ограничениями
		log.Println("Пробуем с минимальными ограничениями...")
		constraints = mediadevices.MediaStreamConstraints{
			Video: func(c *mediadevices.MediaTrackConstraints) {
				// Никаких форматных ограничений
				if *deviceID != "" {
					c.DeviceID = prop.String(*deviceID)
				}
			},
			Codec: mediadevices.NewCodecSelector(
				mediadevices.WithVideoEncoders(&x264Params),
			),
		}

		mediaStream, err = mediadevices.GetUserMedia(constraints)
		if err != nil {
			log.Fatalf("Не удалось получить доступ к медиа-устройству: %v", err)
		}
	}

	defer func() {
		for _, track := range mediaStream.GetTracks() {
			track.Close()
		}
	}()

	// Получаем видеотреки
	videoTracks := mediaStream.GetVideoTracks()
	if len(videoTracks) == 0 {
		log.Fatalf("Видеотрек не обнаружен")
	}
	videoTrack := videoTracks[0]
	log.Printf("Используется камера: %s", videoTrack.ID())

	// Настройка и запуск цикла подключения к серверу
	for {
		// Подключение к серверу
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("Ошибка подключения к серверу: %v. Повторная попытка через 5 секунд...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		close(booted)
		log.Println("Подключено к серверу")

		ctx, cancel := context.WithCancel(context.Background())

		// Захват и отправка видео
		go streamVideo(ctx, conn, videoTrack, *debug)

		// Ожидание сигнала о завершении или ошибки соединения
		select {
		case <-interrupt:
			log.Println("Прерывание получено, закрытие...")
			cancel()
			err := conn.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			if err != nil {
				log.Printf("Ошибка закрытия WebSocket: %v", err)
			}
			conn.Close()
			return
		case <-ctx.Done():
			conn.Close()
			time.Sleep(time.Second)
			break
		}
	}
}

// streamVideo захватывает видео с камеры и отправляет по WebSocket
func streamVideo(ctx context.Context, conn *websocket.Conn, track mediadevices.Track, debug bool) {
	// Получаем кодированные данные из видеотрека (H.264)
	reader, err := track.NewEncodedIOReader("h264")
	if err != nil {
		log.Printf("Ошибка создания ридера: %v", err)
		return
	}
	defer reader.Close()

	log.Println("Начало стриминга видео...")

	frameCounter := 0
	startTime := time.Now()

	// Буфер для чтения данных
	buf := make([]byte, 1024*1024) // 1MB буфер для кадра

	for {
		select {
		case <-ctx.Done():
			log.Println("Стриминг остановлен")
			return
		default:
			// Чтение закодированных данных
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("Ошибка чтения данных: %v", err)
				}
				return
			}

			if n > 0 {
				// Отправка по WebSocket
				err = conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				if err != nil {
					log.Printf("Ошибка отправки данных: %v", err)
					return
				}

				// Отладочная информация
				frameCounter++
				if debug && frameCounter%30 == 0 {
					currentTime := time.Now()
					elapsed := currentTime.Sub(startTime).Seconds()
					fps := float64(frameCounter) / elapsed
					log.Printf("Отправлено фреймов: %d, FPS: %.2f, Размер последнего фрейма: %d байт",
						frameCounter, fps, n)
				}
			}
		}
	}
}
