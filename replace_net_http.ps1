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

# 一時ファイルで置換を実行 (Raw で読み込み、UTF8 を指定)
try {
    $content = Get-Content $temp_file -Raw -Encoding UTF8
    $content = $content -replace [regex]::Escape($search_str), $replace_str
    Set-Content -Path $temp_file -Value $content -Encoding UTF8 -NoNewline
} catch {
    Write-Host "Error during file content replacement in temporary file: $($_.Exception.Message)"
    Remove-Item $temp_file
    exit 1
}

# 置換が成功したか確認
if (Select-String -Path $temp_file -Pattern ([regex]::Escape($search_str)) -Encoding UTF8 -Quiet) {
    Write-Host "Failed to replace strings in temporary file. Content verification failed."
    # デバッグ用に一時ファイルの内容を出力する
    Write-Host "Temporary file content:"
    Get-Content $temp_file -Raw
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
    Write-Host "Failed to copy temporary file to ${server_go_file}: $($_.Exception.Message)"
    Write-Host "Try running this script as Administrator"
    Remove-Item $temp_file
    exit 1
}

# 一時ファイルを削除
Remove-Item $temp_file

Write-Host "Successfully replaced strings in $server_go_file"
