MAKEFLAGS += --no-print-directory

# build executables for current platform
build:
ifeq ($(OS),Windows_NT)
	@echo "HTTPS Server: Building..."
	@go build -ldflags="-s -w" -a -o "akebi-https-server.exe" "./https-server/"
else
	@echo "HTTPS Server: Building..."
	@go build -ldflags="-s -w" -a -o "akebi-https-server" "./https-server/"
	@echo "Keyless Server: Building..."
	@go build -ldflags="-s -w" -a -o "akebi-keyless-server" "./keyless-server/"
endif

# build executables for all platforms
build-all-platforms:
	@echo "HTTPS Server: Building Windows Build..."
	@GOARCH=amd64 GOOS="windows" go build -ldflags="-s -w" -a -o "akebi-https-server.exe" "./https-server/"
	@echo "HTTPS Server: Building Linux (amd64) Build..."
	@GOARCH=amd64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-https-server-amd64" "./https-server/"
	@echo "HTTPS Server: Building Linux (arm64) Build..."
	@GOARCH=arm64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-https-server-arm64" "./https-server/"
	@echo "Keyless Server: Building Linux (amd64) Build..."
	@GOARCH=amd64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-keyless-server-amd64" "./keyless-server/"
	@echo "Keyless Server: Building Linux (arm64) Build..."
	@GOARCH=arm64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-keyless-server-arm64" "./keyless-server/"
