# Makefile для проекта webcam-transfer

# Переменные
SERVER_IMAGE_NAME := webcam-server
SERVER_CONTAINER_NAME := webcam-server-container
SERVER_PORT := 8080
RECORDINGS_DIR := $(shell pwd)/server/recordings

# Клиентская часть
CLIENT_DIR := ./client/cmd/webcam-client

# По умолчанию - вывести справку
.PHONY: all
all: help

.PHONY: help
help:
	@echo "Доступные команды:"
	@echo "  make build-server    - Собрать Docker-образ сервера"
	@echo "  make run-server      - Запустить сервер в Docker-контейнере"
	@echo "  make build-client    - Собрать клиент"
	@echo "  make run-client      - Запустить клиент"
	@echo "  make run-client-list - Запустить клиент и вывести список устройств"
	@echo "  make run-client-device DEVICE_ID=xxx - Запустить клиент с указанным ID устройства"
	@echo "  make stop-server     - Остановить сервер"
	@echo "  make clean           - Очистить ресурсы Docker"
	@echo "  make clean-all       - Полная очистка (включая образы Docker)"

# Сборка Docker-образа сервера
.PHONY: build-server
build-server:
	@echo "Сборка Docker-образа сервера..."
	docker build -t $(SERVER_IMAGE_NAME) ./server

# Запуск сервера в Docker-контейнере
.PHONY: run-server
run-server: build-server
	@echo "Запуск сервера на порту $(SERVER_PORT)..."
	docker run -d --name $(SERVER_CONTAINER_NAME) \
		-p $(SERVER_PORT):$(SERVER_PORT) \
		-v $(RECORDINGS_DIR):/app/recordings \
		$(SERVER_IMAGE_NAME)
	@echo "Сервер запущен, доступен по адресу: http://localhost:$(SERVER_PORT)"

# Сборка клиента
.PHONY: build-client
build-client:
	@echo "Сборка клиента..."
	cd $(CLIENT_DIR) && go build -o ../../../webcam-client

# Запуск клиента
.PHONY: run-client
run-client: build-client
	@echo "Запуск клиента..."
	./webcam-client --address localhost:$(SERVER_PORT)

# Запуск клиента в режиме вывода списка устройств
.PHONY: run-client-list
run-client-list: build-client
	@echo "Вывод списка доступных устройств..."
	./webcam-client --list-devices

# Запуск клиента с указанным ID устройства
.PHONY: run-client-device
run-client-device: build-client
	@if [ -z "$(DEVICE_ID)" ]; then \
		echo "Ошибка: Не указан ID устройства. Используйте: make run-client-device DEVICE_ID=xxx"; \
		exit 1; \
	fi
	@echo "Запуск клиента с устройством $(DEVICE_ID)..."
	./webcam-client --address localhost:$(SERVER_PORT) --device-id $(DEVICE_ID)

# Остановка сервера
.PHONY: stop-server
stop-server:
	@echo "Остановка сервера..."
	-docker stop $(SERVER_CONTAINER_NAME)
	-docker rm $(SERVER_CONTAINER_NAME)

# Очистка Docker ресурсов
.PHONY: clean
clean: stop-server
	@echo "Очистка Docker ресурсов..."
	-docker container prune -f

# Полная очистка (включая образы)
.PHONY: clean-all
clean-all: clean
	@echo "Удаление Docker-образа сервера..."
	-docker rmi $(SERVER_IMAGE_NAME)
	@echo "Удаление собранного клиента..."
	-rm -f webcam-client

# Создание директории для записей, если она не существует
$(RECORDINGS_DIR):
	mkdir -p $(RECORDINGS_DIR)

# Зависимость для запуска сервера
run-server: $(RECORDINGS_DIR) 