#!/bin/bash

# HTTPS サーバーに HTTP でアクセスした際に出力される HTML は、残念ながら Golang 標準ライブラリの net/http/server.go にハードコードされている
# このためにわざわざフォークを作るのも面倒なので、sed でソース自体を強引に置換するためのスクリプト (with GPT-4)

# net/http/server.go のフルパス
go_path=$(go env GOROOT)
server_go_file="${go_path}/src/net/http/server.go"

# ファイルが存在することを確認
if [ ! -f "$server_go_file" ]; then
  echo "Error: File $server_go_file does not exist"
  exit 1
fi

# 置換する文字列
search_str="Client sent an HTTP request to an HTTPS server."

# 置換後のエスケープ済み HTML
replace_str="<!DOCTYPE html><html><head><meta charset=\\'UTF-8\\'><title>Automatically jump to HTTPS</title><script>window.location.replace(window.location.href.replace(\\'http:\\/\\/', \\'https:\\/\\/'));<\\/script><\\/head><body><\\/body><\\/html>"

# 一時ファイルを作成
temp_file=$(mktemp)

# ファイルを一時ファイルにコピー
cp "$server_go_file" "$temp_file"

# 一時ファイルで置換を実行
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS
  sed -i '' "s|${search_str}|${replace_str}|g" "$temp_file"
else
  # Linux
  sed -i "s|${search_str}|${replace_str}|g" "$temp_file"
fi

# 置換が成功したか確認
if grep -q "${search_str}" "$temp_file"; then
  echo "Failed to replace strings in temporary file"
  rm "$temp_file"
  exit 1
fi

# 元のファイルに書き込み権限を付与
chmod u+w "$server_go_file" 2>/dev/null || true

# 一時ファイルを元の場所にコピー
if ! cp "$temp_file" "$server_go_file"; then
  echo "Failed to copy temporary file to ${server_go_file}"
  echo "Try running this script with sudo"
  rm "$temp_file"
  exit 1
fi

# 一時ファイルを削除
rm "$temp_file"

echo "Successfully replaced strings in ${server_go_file}"
