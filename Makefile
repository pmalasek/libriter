# Libriter – build orchestrace
#
# Frontend se sestavuje přímo do libriter-backend/internal/web/dist a odtud
# se přes go:embed vkompiluje do binárky. `make backend` funguje i bez
# sestaveného frontendu – server pak na / vrací stránku s návodem (503).

BIN       := bin/libriter
FRONTEND  := libriter-frontend
BACKEND   := libriter-backend
DIST      := $(BACKEND)/internal/web/dist

.PHONY: help build frontend backend dev-backend dev-frontend vet clean

help:
	@echo "Cíle:"
	@echo "  make build         sestaví frontend i backend do $(BIN)"
	@echo "  make frontend      npm ci && npm run build (výstup do $(DIST))"
	@echo "  make backend       go build -o $(BIN) (vkompiluje aktuální $(DIST))"
	@echo "  make dev-backend   go run ./cmd/server (vývojový server na :8080)"
	@echo "  make dev-frontend  npm run dev (Vite na :5173, proxy /api na :8080)"
	@echo "  make vet           go vet ./..."
	@echo "  make clean         smaže $(BIN) a obsah $(DIST)"

build: frontend backend

frontend:
	cd $(FRONTEND) && npm ci && npm run build

backend:
	mkdir -p bin
	cd $(BACKEND) && CGO_ENABLED=0 go build -o ../$(BIN) ./cmd/server

dev-backend:
	cd $(BACKEND) && go run ./cmd/server

dev-frontend:
	cd $(FRONTEND) && npm run dev

vet:
	cd $(BACKEND) && go vet ./...

clean:
	rm -rf bin
	find $(DIST) -mindepth 1 ! -name .gitkeep -delete
