.PHONY: test lint run build docker-up docker-down smoke tidy

HTTP_HOST ?= 127.0.0.1
HTTP_PORT ?= 8875
RTSP_HOST ?= 127.0.0.1
RTSP_PORT ?= 554

test:
	go test ./...

lint:
	go vet ./...

tidy:
	go mod tidy

build:
	go build -o bin/satip-lab ./cmd/satip-lab
	go build -o bin/satip-lab-mcp ./cmd/satip-lab-mcp
	go build -o bin/satip-lab-smoke ./cmd/satip-lab-smoke
	go build -o bin/satip-labctl ./cmd/satip-labctl

run:
	go run ./cmd/satip-lab

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

smoke:
	curl -fsS http://$(HTTP_HOST):$(HTTP_PORT)/desc.xml | grep -q 'SatIPServer'
	curl -fsS http://$(HTTP_HOST):$(HTTP_PORT)/channels.m3u | grep -q 'ZDF HD'
	go run ./cmd/satip-lab-smoke --host $(RTSP_HOST) --rtsp-port $(RTSP_PORT)
	@echo "smoke OK"
