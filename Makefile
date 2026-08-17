BINARY      := mac-cleaner
PKG         := ./cmd/mac-cleaner
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo devel)
LDFLAGS     := -X github.com/danfigueroa/mac-cleaner/internal/cli.version=$(VERSION)
LINT        := bin/golangci-lint

.DEFAULT_GOAL := help
.PHONY: help build install test test-race cover lint fmt tidy clean run

help: ## Lista os alvos disponíveis
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk -F':.*?## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Compila o binário em bin/
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

install: ## Instala em $(GOPATH)/bin (já está no PATH)
	go install -ldflags '$(LDFLAGS)' $(PKG)

run: ## Roda a CLI sem instalar (use ARGS="report --json")
	go run -ldflags '$(LDFLAGS)' $(PKG) $(ARGS)

test: ## Roda os testes
	go test ./...

test-race: ## Roda os testes com o detector de corrida
	go test -race ./...

cover: ## Gera e abre o relatório de cobertura
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# O linter é pinado no módulo tools/ para que CI e máquina local rodem
# exatamente a mesma versão, sem depender do que cada um instalou via brew.
$(LINT): tools/go.mod
	go -C tools build -o ../$(LINT) github.com/golangci/golangci-lint/v2/cmd/golangci-lint

lint: $(LINT) ## Roda o linter
	$(LINT) run ./...

fmt: $(LINT) ## Formata o código (gofumpt + gci)
	$(LINT) fmt ./...

tidy: ## Arruma os dois módulos
	go mod tidy
	go -C tools mod tidy

clean: ## Remove artefatos de build
	rm -rf bin coverage.out
