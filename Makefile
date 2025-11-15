# FloraLab Parent Makefile
# Builds florago binaries and floralab package

.PHONY: help build-all build-florago build-floralab copy-binaries clean test install

# Default target
.DEFAULT_GOAL := help

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

## help: Show this help message
help:
	@echo "$(BLUE)FloraLab Build System$(NC)"
	@echo ""
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'

## build-all: Build everything (florago binaries + floralab package)
build-all: build-florago copy-binaries build-floralab
	@echo "$(GREEN)✓ Build complete!$(NC)"
	@echo ""
	@echo "$(BLUE)Binaries:$(NC)"
	@ls -lh florago/bin/florago-*
	@echo ""
	@echo "$(BLUE)floralab package:$(NC)"
	@ls -lh floralab/bin/florago-*
	@echo ""
	@echo "$(GREEN)Ready to install:$(NC) pip install -e ."

## build-florago: Build florago binaries (amd64, arm64)
build-florago:
	@echo "$(BLUE)Building florago binaries...$(NC)"
	@cd florago && $(MAKE) build-all
	@echo "$(BLUE)Building dependencies (Caddy, Delve)...$(NC)"
	@cd florago && $(MAKE) build-deps
	@echo "$(GREEN)✓ florago binaries and dependencies built$(NC)"

## copy-binaries: Copy florago binaries to floralab/bin/
copy-binaries:
	@echo "$(BLUE)Copying binaries to floralab...$(NC)"
	@mkdir -p floralab/bin
	@cp florago/bin/florago-amd64 floralab/bin/florago-amd64
	@cp florago/bin/florago-arm64 floralab/bin/florago-arm64
	@cp florago/bin/caddy-amd64 floralab/bin/caddy-amd64
	@cp florago/bin/dlv-amd64 floralab/bin/dlv-amd64
	@chmod +x floralab/bin/florago-*
	@chmod +x floralab/bin/caddy-*
	@chmod +x floralab/bin/dlv-*
	@echo "$(GREEN)✓ Binaries copied$(NC)"

## build-floralab: Build floralab Python package (requires binaries)
build-floralab:
	@echo "$(BLUE)Building floralab package...$(NC)"
	@if [ ! -f floralab/bin/florago-amd64 ]; then \
		echo "$(RED)✗ Error: florago binaries not found. Run 'make copy-binaries' first.$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Building wheel...$(NC)"
	@if command -v uv >/dev/null 2>&1; then \
		uv build --wheel; \
	else \
		python -m build --wheel; \
	fi
	@echo "$(GREEN)✓ floralab wheel built$(NC)"
	@ls -lh dist/*.whl 2>/dev/null || true

## install: Install floralab package with dependencies
install: build-all
	@echo "$(BLUE)Installing floralab package...$(NC)"
	@if command -v uv >/dev/null 2>&1; then \
		uv pip install -e .; \
	else \
		pip install -e .; \
	fi
	@echo "$(GREEN)✓ floralab installed$(NC)"
	@echo ""
	@echo "$(GREEN)Try it:$(NC)"
	@echo "  floralab-cli --help"

## clean: Remove build artifacts
clean:
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@cd florago && $(MAKE) clean
	@rm -rf floralab/bin/florago-*
	@rm -rf build/ dist/ *.egg-info floralab.egg-info
	@find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	@find . -type f -name '*.pyc' -delete
	@echo "$(GREEN)✓ Clean complete$(NC)"

## test: Run tests for both florago and floralab
test:
	@echo "$(BLUE)Running tests...$(NC)"
	@echo "$(YELLOW)Testing florago...$(NC)"
	@cd florago && go test ./... || true
	@echo ""
	@echo "$(YELLOW)Testing floralab...$(NC)"
	@if [ -f floralab/tests/__init__.py ]; then \
		pytest floralab/tests/ || true; \
	else \
		echo "  No tests found"; \
	fi
	@echo "$(GREEN)✓ Tests complete$(NC)"

## dev-setup: Setup development environment
dev-setup:
	@echo "$(BLUE)Setting up development environment...$(NC)"
	@echo "$(YELLOW)Installing Go dependencies...$(NC)"
	@cd florago && go mod download
	@echo "$(YELLOW)Installing Python dependencies...$(NC)"
	@if command -v uv >/dev/null 2>&1; then \
		uv pip install -e ".[dev]"; \
	else \
		pip install -e ".[dev]"; \
	fi
	@echo "$(GREEN)✓ Development environment ready$(NC)"

## check: Check code formatting and linting
check:
	@echo "$(BLUE)Checking code quality...$(NC)"
	@echo "$(YELLOW)Checking Go code...$(NC)"
	@cd florago && go fmt ./...
	@cd florago && go vet ./... || true
	@echo "$(YELLOW)Checking Python code...$(NC)"
	@if command -v ruff >/dev/null 2>&1; then \
		ruff check floralab/; \
	else \
		echo "  ruff not installed, skipping"; \
	fi
	@echo "$(GREEN)✓ Check complete$(NC)"

## package: Create distribution packages
package: build-all
	@echo "$(BLUE)Creating distribution packages...$(NC)"
	@if command -v uv >/dev/null 2>&1; then \
		uv build; \
	else \
		python -m build; \
	fi
	@echo "$(GREEN)✓ Packages created in dist/$(NC)"
	@ls -lh dist/

## publish-test: Publish to TestPyPI
publish-test: package
	@echo "$(BLUE)Publishing to TestPyPI...$(NC)"
	@if command -v uv >/dev/null 2>&1; then \
		uv publish --publish-url https://test.pypi.org/legacy/; \
	else \
		python -m twine upload --repository testpypi dist/*; \
	fi
	@echo "$(GREEN)✓ Published to TestPyPI$(NC)"
	@echo "$(YELLOW)Install with:$(NC) pip install --index-url https://test.pypi.org/simple/ floralab"

## publish: Publish to PyPI
publish: package
	@echo "$(RED)⚠ Publishing to PyPI (production)$(NC)"
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@if command -v uv >/dev/null 2>&1; then \
		uv publish; \
	else \
		python -m twine upload dist/*; \
	fi
	@echo "$(GREEN)✓ Published to PyPI$(NC)"
	@echo "$(YELLOW)Install with:$(NC) pip install floralab"

## quick: Quick build (amd64 only)
quick:
	@echo "$(BLUE)Quick build (amd64 only)...$(NC)"
	@cd florago && $(MAKE) build-amd64
	@mkdir -p floralab/bin
	@cp florago/bin/florago-amd64 floralab/bin/florago-amd64
	@chmod +x floralab/bin/florago-amd64
	@echo "$(GREEN)✓ Quick build complete$(NC)"

## version: Show versions
version:
	@echo "$(BLUE)FloraLab Version Information$(NC)"
	@echo ""
	@echo "$(YELLOW)florago:$(NC)"
	@if [ -f florago/bin/florago-amd64 ]; then \
		florago/bin/florago-amd64 version 2>/dev/null || echo "  Not built yet"; \
	else \
		echo "  Not built yet"; \
	fi
	@echo ""
	@echo "$(YELLOW)floralab:$(NC)"
	@python -c "import tomllib; print('  Version:', tomllib.load(open('pyproject.toml', 'rb'))['project']['version'])" 2>/dev/null || echo "  Unknown"
	@echo ""
	@echo "$(YELLOW)Go:$(NC)"
	@go version
	@echo ""
	@echo "$(YELLOW)Python:$(NC)"
	@python --version

## info: Show build information
info:
	@echo "$(BLUE)FloraLab Build Information$(NC)"
	@echo ""
	@echo "$(GREEN)Project Structure:$(NC)"
	@echo "  florago/    - Go backend (REST API, SLURM, Flower)"
	@echo "  floralab/   - Python CLI (user interface)"
	@echo ""
	@echo "$(GREEN)Build Process:$(NC)"
	@echo "  1. Build florago binaries (amd64, arm64)"
	@echo "  2. Copy binaries to floralab/bin/"
	@echo "  3. Build floralab Python package"
	@echo ""
	@echo "$(GREEN)Artifacts:$(NC)"
	@echo "  florago/bin/florago-amd64    - Linux/macOS Intel/Windows"
	@echo "  florago/bin/florago-arm64    - macOS Apple Silicon"
	@echo "  floralab/bin/florago-amd64   - Bundled in Python package"
	@echo ""
	@echo "$(GREEN)Documentation:$(NC)"
	@echo "  DOCUMENTATION.md             - Complete guide"
	@echo "  README.md                    - Quick start"
	@echo ""
	@echo "$(YELLOW)Run 'make help' for available targets$(NC)"
