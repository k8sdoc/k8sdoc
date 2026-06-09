VERSION    ?= 0.1.0
IMAGE_REPO ?= ghcr.io/user/k8sdoc
BINARY     := k8sdoc

.PHONY: all build run test lint fmt docker-build helm-install setup-ollama

all: fmt lint build

build:
	go build -ldflags="-X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)" \
	  -o bin/$(BINARY) ./

run:
	go run ./main.go analyze --explain

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

docker-build:
	docker build -t $(IMAGE_REPO):$(VERSION) -t $(IMAGE_REPO):latest .

docker-push:
	docker push $(IMAGE_REPO):$(VERSION)
	docker push $(IMAGE_REPO):latest

setup-ollama:
	@echo "Starting Ollama with CUDA support..."
	OLLAMA_CUDA=1 ollama serve &
	@sleep 3
	@echo "Pulling Qwen2.5-Coder 14B (q4_K_M quantization)..."
	ollama pull qwen2.5-coder:14b-instruct-q4_K_M
	@echo "Done! Run 'make run' to test."

auth-ollama:
	./bin/$(BINARY) auth add \
	  --backend ollama \
	  --baseurl http://localhost:11434 \
	  --model qwen2.5-coder:14b-instruct-q4_K_M \
	  --default

helm-install:
	helm upgrade --install k8sdoc ./charts/k8sdoc \
	  --namespace k8sdoc --create-namespace \
	  --values ./charts/k8sdoc/values.yaml

helm-uninstall:
	helm uninstall k8sdoc --namespace k8sdoc

deps:
	go mod tidy
	go mod download
