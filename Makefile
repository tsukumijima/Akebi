MAKEFLAGS += --no-print-directory

# build executables for current platform
build-https-server:
	@echo "HTTPS Server: Building..."
ifeq ($(OS),Windows_NT)
	@powershell replace_net_http.ps1
	@go build -ldflags="-s -w" -a -o "akebi-https-server.exe" "./https-server/"
else
	@sudo bash replace_net_http.sh
	@go build -ldflags="-s -w" -a -o "akebi-https-server" "./https-server/"
endif

# build executables for all platforms
build-https-server-all-platforms:
ifeq ($(OS),Windows_NT)
	@powershell replace_net_http.ps1
else
	@sudo bash replace_net_http.sh
endif
	@echo "HTTPS Server: Building Windows Build..."
	@GOARCH=amd64 GOOS="windows" go build -ldflags="-s -w" -a -o "akebi-https-server.exe" "./https-server/"
	@echo "HTTPS Server: Building Linux (x64) Build..."
	@GOARCH=amd64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-https-server" "./https-server/"
	@echo "HTTPS Server: Building Linux (arm64) Build..."
	@GOARCH=arm64 GOOS="linux" go build -ldflags="-s -w" -a -o "akebi-https-server-arm" "./https-server/"

# currently, linux and systemd combination only
build-keyless-server:
	@echo "Keyless Server: Building..."
	@go build -ldflags="-s -w" -a -o "akebi-keyless-server" "./keyless-server/"
