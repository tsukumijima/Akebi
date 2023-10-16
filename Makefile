MAKEFLAGS += --no-print-directory

# build executables for current platform
# On Linux, depending on where Golang is installed, root privileges may be required to replace net/http
build-https-server:
	@echo "HTTPS Server: Building..."
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy RemoteSigned -File .\replace_net_http.ps1
	@go build -ldflags="-s -w" -a -v -o "akebi-https-server.exe" "./https-server/"
else
	@bash ./replace_net_http.sh
	@go build -ldflags="-s -w" -a -v -o "akebi-https-server" "./https-server/"
endif

# build executables for all platforms
# On Linux, depending on where Golang is installed, root privileges may be required to replace net/http
build-https-server-all-platforms:
ifeq ($(OS),Windows_NT)
	@powershell -ExecutionPolicy RemoteSigned -File .\replace_net_http.ps1
else
	@bash ./replace_net_http.sh
endif
	@echo "HTTPS Server: Building Windows Build..."
	@GOARCH=amd64 GOOS="windows" go build -ldflags="-s -w" -a -v -o "akebi-https-server.exe" "./https-server/"
	@echo "HTTPS Server: Building Linux (x64) Build..."
	@GOARCH=amd64 GOOS="linux" go build -ldflags="-s -w" -a -v -o "akebi-https-server" "./https-server/"
	@echo "HTTPS Server: Building Linux (arm64) Build..."
	@GOARCH=arm64 GOOS="linux" go build -ldflags="-s -w" -a -v -o "akebi-https-server-arm" "./https-server/"

# currently, linux and systemd combination only
build-keyless-server:
	@echo "Keyless Server: Building..."
	@go build -ldflags="-s -w" -a -v -o "akebi-keyless-server" "./keyless-server/"
