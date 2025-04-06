package domain

// VideoFrame представляет кадр видео
type VideoFrame struct {
	Data   []byte // Данные кадра в H.264 формате
	Size   int    // Размер данных в байтах
	Number int    // Номер кадра
}

// VideoDevice представляет устройство захвата видео
type VideoDevice struct {
	ID    string // Уникальный идентификатор устройства
	Label string // Человекочитаемое имя устройства
	Kind  string // Тип устройства
}

// VideoConfig содержит конфигурацию видеопотока
type VideoConfig struct {
	Width        int    // Ширина видео в пикселях
	Height       int    // Высота видео в пикселях
	FrameRate    int    // Частота кадров
	BitRate      int    // Битрейт в bps
	DeviceID     string // ID устройства для захвата
	CodecName    string // Имя кодека (например, "h264")
	StreamingURL string // URL для стриминга
}

// VideoReader интерфейс для чтения видеокадров
type VideoReader interface {
	Read() (*VideoFrame, error)
	Close() error
}

// VideoStreamer интерфейс для стриминга видео
type VideoStreamer interface {
	Connect() error
	Disconnect() error
	SendFrame(frame *VideoFrame) error
	IsConnected() bool
}

// VideoTrack представляет видеотрек
type VideoTrack interface {
	ID() string
	Close() error
	CreateReader() (VideoReader, error)
}
