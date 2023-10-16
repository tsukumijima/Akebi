
# HTTPS サーバーに HTTP でアクセスした際に出力される HTML は、残念ながら Golang 標準ライブラリの net/http/server.go にハードコードされている
# このためにわざわざフォークを作るのも面倒なので、PowerShell でソース自体を強引に置換するためのスクリプト (with GPT-4)

# net/http/server.go のフルパスを取得
$go_path = & go env GOROOT
$server_go_file = "${go_path}\src\net\http\server.go"

# 置換する文字列
$search_str = "Client sent an HTTP request to an HTTPS server."

# 置換後のエスケープ済み HTML
$replace_str = "<!DOCTYPE html><html><head><meta charset='UTF-8'><title>Automatically jump to HTTPS</title><script>window.location.replace(window.location.href.replace('http://', 'https://'));</script></head><body></body></html>"

# 文字列を置換
(Get-Content $server_go_file) -replace [regex]::Escape($search_str), $replace_str | Set-Content $server_go_file

# 実際に置換されたかどうか確認するために、置換前の文字列がないことを確認する
if ((Get-Content $server_go_file) -match $search_str) {
    Write-Host "Failed to replace strings in $server_go_file"
    exit 1
}
Write-Host "Successfully replaced strings in $server_go_file"
