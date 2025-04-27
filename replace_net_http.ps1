# HTTPS サーバーに HTTP でアクセスした際に出力される HTML は、残念ながら Golang 標準ライブラリの net/http/server.go にハードコードされている
# このためにわざわざフォークを作るのも面倒なので、PowerShell でソース自体を強引に置換するためのスクリプト (with GPT-4)

# net/http/server.go のフルパスを取得
$go_path = & go env GOROOT
$server_go_file = "${go_path}\src\net\http\server.go"

# ファイルが存在することを確認
if (-not (Test-Path $server_go_file)) {
    Write-Host "Error: File $server_go_file does not exist"
    exit 1
}

# 置換する文字列
$search_str = "Client sent an HTTP request to an HTTPS server."

# 置換後のエスケープ済み HTML
$replace_str = "<!DOCTYPE html><html><head><meta charset='UTF-8'><title>Automatically jump to HTTPS</title><script>window.location.replace(window.location.href.replace('http://', 'https://'));</script></head><body></body></html>"

# 一時ファイルを作成
$temp_file = [System.IO.Path]::GetTempFileName()

# ファイルを一時ファイルにコピー
Copy-Item -Path $server_go_file -Destination $temp_file -Force

# 一時ファイルで置換を実行
$content = Get-Content $temp_file
$content = $content -replace [regex]::Escape($search_str), $replace_str
$content | Set-Content $temp_file

# 置換が成功したか確認
if (Select-String -Path $temp_file -Pattern $search_str -Quiet) {
    Write-Host "Failed to replace strings in temporary file"
    Remove-Item $temp_file
    exit 1
}

# 元のファイルの読み取り専用属性を解除
try {
    Set-ItemProperty -Path $server_go_file -Name IsReadOnly -Value $false -ErrorAction SilentlyContinue
} catch {
    # エラーを無視
}

# 一時ファイルを元の場所にコピー
try {
    Copy-Item -Path $temp_file -Destination $server_go_file -Force
} catch {
    Write-Host "Failed to copy temporary file to $server_go_file"
    Write-Host "Try running this script as Administrator"
    Remove-Item $temp_file
    exit 1
}

# 一時ファイルを削除
Remove-Item $temp_file

Write-Host "Successfully replaced strings in $server_go_file"
