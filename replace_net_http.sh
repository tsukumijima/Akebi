#!/bin/bash

# HTTPS サーバーに HTTP でアクセスした際に出力される HTML は、残念ながら Golang 標準ライブラリの net/http/server.go にハードコードされている
# このためにわざわざフォークを作るのも面倒なので、sed でソース自体を強引に置換するためのスクリプト (with GPT-4)

# net/http/server.go のフルパス
go_path=$(go env GOROOT)
server_go_file="${go_path}/src/net/http/server.go"

# 置換する文字列
search_str="Client sent an HTTP request to an HTTPS server."

# 置換後のエスケープ済み HTML
replace_str="<!DOCTYPE html><html><head><meta charset=\\'UTF-8\\'><title>Automatically jump to HTTPS</title><script>window.location.replace(window.location.href.replace(\\'http:\\/\\/', \\'https:\\/\\/'));<\\/script><\\/head><body><\\/body><\\/html>"

# 文字列を置換 (GNU sed と BSD sed の両方に対応)
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS
  sed -i '' "s|${search_str}|${replace_str}|g" "$server_go_file"
else
  # Linux
  sed -i "s|${search_str}|${replace_str}|g" "$server_go_file"
fi

# 実際に置換されたかどうか確認するために、置換前の文字列がないことを確認する
if grep -q "${search_str}" "${server_go_file}"; then
  echo "Failed to replace strings in ${server_go_file}"
  exit 1
fi
echo "Successfully replaced strings in ${server_go_file}"
