# omoikane-go

`github.com/ieee0824/omoikane-go` は、[Omoikane](https://github.com/ieee0824/omoikane) の C ABI を Go から扱うためのラッパーです。

## 仕組み

- `omoikane` 本体の GitHub Releases から共有ライブラリを取得します
- macOS と Linux の `amd64` / `arm64` を対象にしています
- 既定では `v0.2.2` を使います
- `OMOIKANE_LIBRARY_PATH` を設定すると、ローカルの `.dylib` / `.so` を優先して読み込みます

## インストール

```bash
go get github.com/ieee0824/omoikane-go
```

## 使い方

```go
package main

import (
	"fmt"

	omoikane "github.com/ieee0824/omoikane-go"
)

func main() {
	browser, err := omoikane.NewBrowser(omoikane.Options{
		UserAgent: "omoikane-go-example/0.2.2",
	})
	if err != nil {
		panic(err)
	}
	defer browser.Close()

	if err := browser.Navigate(`data:text/html,<html><body><main id="app">hello</main></body></html>`); err != nil {
		panic(err)
	}

	result, err := browser.Evaluate(`document.getElementById("app").nodeName`)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(result))
}
```

## バージョン固定

```go
browser, err := omoikane.NewBrowser(omoikane.Options{
	Version: "v0.2.2",
})
```

## User-Agent を指定する

```go
browser, err := omoikane.NewBrowser(omoikane.Options{
	Version:   "v0.2.2",
	UserAgent: "my-app/1.0",
})
```

## サンプル

```bash
go run ./examples/visit_localhost
go run ./examples/get_ogp_image https://example.com
```

## ローカルビルドを使う

```bash
export OMOIKANE_LIBRARY_PATH=/path/to/libomoikane.dylib
go test ./...
```

## テスト

```bash
go test ./...
```

ローカルの `omoikane` ビルドを使って統合テストを回す場合は、次のどちらかを設定します。

```bash
export OMOIKANE_LIBRARY_PATH=/path/to/libomoikane.dylib
```

または

```bash
export OMOIKANE_RUST_REPO=/path/to/omoikane
```
