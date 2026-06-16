.PHONY: help dev build build-ui build-desktop run clean test lint lint-go lint-ui \
	format format-go format-ui check swagger setup install-hooks docker-build docker-up docker-down \
	e2e e2e-install e2e-headed e2e-report e2e-docker-up e2e-docker-down

help:
	@echo "AIOps Makefile"
	@echo ""
	@echo "开发命令:"
	@echo "  make dev          启动后端开发服务器 (hot-reload)"
	@echo "  make build        编译后端 + 构建前端"
	@echo "  make build-ui     构建前端 (Quasar SPA)"
	@echo "  make build-desktop  构建桌面客户端 (Tauri)"
	@echo "  make run          运行编译后的后端"
	@echo "  make clean        清理构建产物"
	@echo ""
	@echo "代码质量:"
	@echo "  make lint         代码检查 (Go + UI)"
	@echo "  make lint-go      Go 代码检查 (golangci-lint)"
	@echo "  make lint-ui      前端代码检查 (ESLint)"
	@echo "  make format       格式化代码 (Go + UI)"
	@echo "  make format-go    Go 代码格式化 (gofmt)"
	@echo "  make format-ui    前端代码格式化 (Prettier)"
	@echo "  make check        完整检查 (format + lint + test)"
	@echo ""
	@echo "测试:"
	@echo "  make test         运行 Go 测试"
	@echo "  make swagger      生成 Swagger 文档"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build 构建 Docker 镜像"
	@echo "  make docker-up    启动 Docker 服务"
	@echo "  make docker-down  停止 Docker 服务"
	@echo ""
	@echo "项目设置:"
	@echo "  make setup        安装开发依赖 + Git hooks"
	@echo "  make install-hooks  安装 Git hooks (husky)"

dev:
	@echo "启动后端开发服务器..."
	@cp -n .env.example .env 2>/dev/null || true
	go run ./cmd/server

build: build-ui
	@echo "编译后端..."
	@mkdir -p build
	go build -o build/aiops ./cmd/server

build-ui:
	@echo "构建前端..."
	cd ui && npm run build

build-desktop:
	@echo "构建桌面客户端..."
	cd aiops-desktop && npm run tauri build

run:
	@echo "运行后端..."
	./build/aiops

test:
	@echo "运行测试..."
	go test ./... -v -count=1

lint: lint-go lint-ui

lint-go:
	@echo "Go 代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

lint-ui:
	@echo "前端代码检查..."
	cd ui && npm run lint

format: format-go format-ui

format-go:
	@echo "Go 代码格式化..."
	gofmt -w -s .

format-ui:
	@echo "前端代码格式化..."
	@if [ -d node_modules ] || [ -d ui/node_modules ]; then \
		npx prettier --write "ui/**/*.{js,ts,vue,css,scss,json,md}"; \
	else \
		echo "Prettier 未安装，请先执行 make setup"; \
	fi

check: format lint test
	@echo "所有检查通过 ✓"

swagger:
	@echo "生成 Swagger 文档..."
	@bash scripts/regen-swagger.sh

setup:
	@echo "安装前端依赖..."
	cd ui && npm install
	@echo "安装开发工具依赖..."
	npm install
	@echo "初始化 Git hooks..."
	npx husky
	@echo "设置完成！请执行 git init 初始化仓库（如果尚未初始化）"

install-hooks:
	@echo "安装 Git hooks..."
	npx husky

clean:
	@echo "清理构建产物..."
	rm -rf build/
	rm -rf ui/dist/
	rm -rf aiops-desktop/src-tauri/target/
	rm -f aiops

docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t aiops:latest .

docker-up:
	@echo "启动 Docker 服务..."
	docker compose up -d

docker-down:
	@echo "停止 Docker 服务..."
	docker compose down

# ──────────────────────────────────────────────
# E2E 测试 (Playwright)
# ──────────────────────────────────────────────

E2E_DIR := e2e

e2e-install:
	@echo "安装 Playwright 和浏览器..."
	cd $(E2E_DIR) && npm install && npx playwright install chromium

e2e:
	@echo "运行 E2E 测试 (headless)..."
	cd $(E2E_DIR) && npx playwright test

e2e-headed:
	@echo "运行 E2E 测试 (有头模式)..."
	cd $(E2E_DIR) && npx playwright test --headed

e2e-report:
	@echo "打开 E2E 测试报告..."
	cd $(E2E_DIR) && npx playwright show-report

e2e-docker-up:
	@echo "启动 E2E 测试环境 (Docker Compose)..."
	docker compose -f $(E2E_DIR)/docker-compose.e2e.yml up -d --build
	@echo "等待 app 就绪..."
	@sleep 5
	@echo "App 就绪，可运行 'make e2e' 执行测试"

e2e-docker-down:
	@echo "停止 E2E 测试环境..."
	docker compose -f $(E2E_DIR)/docker-compose.e2e.yml down -v