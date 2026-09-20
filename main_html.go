package main

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/net/html"
)

// extractMainHTML は最初の main 要素を開始・終了タグごと元のバイト列から切り出す。
// DOM の再出力や文字列変換を行わないため、属性・文字参照・改行をそのまま保持する。
// template と SVG/MathML の内部は対象外とし、HTML の main の入れ子は受け付けない。
func extractMainHTML(source []byte) ([]byte, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	offset, mainStart := 0, -1
	var excludedElements []string
	foreignDepth := 0

	for {
		// 外来要素の CDATA 内にあるタグ風の文字列も抽出対象にしない。
		tokenizer.AllowCDATA(foreignDepth > 0)
		tokenType := tokenizer.Next()
		tokenStart := offset
		// TagName は内部バッファを小文字化するので、その前に Raw の長さを取得する。
		// 出力には Tokenizer のバッファではなく、変更していない source を使う。
		offset += len(tokenizer.Raw())

		if tokenType == html.ErrorToken {
			if err := tokenizer.Err(); err != io.EOF {
				return nil, fmt.Errorf("HTMLの解析に失敗しました: %w", err)
			}
			if mainStart >= 0 {
				return nil, fmt.Errorf("main要素の終了タグが見つかりませんでした")
			}
			return nil, fmt.Errorf("main要素が見つかりませんでした")
		}

		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken && tokenType != html.EndTagToken {
			continue
		}
		tagName, _ := tokenizer.TagName()
		name := string(tagName)

		if tokenType == html.EndTagToken {
			if len(excludedElements) > 0 {
				if name == excludedElements[len(excludedElements)-1] {
					excludedElements = excludedElements[:len(excludedElements)-1]
					if name == "svg" || name == "math" {
						foreignDepth--
					}
				}
				continue
			}
			if name != "main" {
				continue
			}
			if mainStart < 0 {
				return nil, fmt.Errorf("開始タグのないmain終了タグが見つかりました")
			}
			return bytes.Clone(source[mainStart:offset]), nil
		}

		switch name {
		case "template", "svg", "math":
			// HTML の template では自己終了のスラッシュは終了タグの代わりにならない。
			if tokenType == html.StartTagToken || name == "template" {
				excludedElements = append(excludedElements, name)
				if name == "svg" || name == "math" {
					foreignDepth++
				}
			}
			continue
		}
		if len(excludedElements) > 0 || name != "main" {
			continue
		}
		if tokenType == html.SelfClosingTagToken {
			return nil, fmt.Errorf("main要素の自己終了タグには対応していません")
		}
		if mainStart >= 0 {
			return nil, fmt.Errorf("main要素が入れ子になっています")
		}
		mainStart = tokenStart
	}
}
