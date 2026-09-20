package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExtractMainHTML_PreservesOriginalBytes(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		main   string
		suffix string
	}{
		{
			name:   "属性と大文字タグとCRLFと文字参照とルビ",
			prefix: "\xef\xbb\xbf<!DOCTYPE html>\r\n<html><head><title>作品</title></head><body>\r\n",
			main:   "<MAIN\r\n class = 'p-novel-main' data-note=\"<main> &amp; >\">\r\n  <ruby>漢字<rp>（</rp><rt>かんじ</rt><rp>）</rp></ruby>&#x3000;&nbsp;\r\n</MaIn >",
			suffix: "\r\n<footer>末尾</footer></body></html>",
		},
		{
			name:   "コメントとscriptとstyleとtextarea内の偽タグ",
			prefix: `<!-- <main>コメント</main> --><script>const a = "<main>script</main>";</script><style>x::before { content: "<main>style</main>"; }</style><textarea><main>textarea</main></textarea>`,
			main:   `<main><!-- </main> --><script>const b = "</main>";</script>本文</main>`,
		},
		{
			name:   "テンプレート内のmainは対象外",
			prefix: `<template><main>雛形</main><template><main>入れ子の雛形</main></template></template>`,
			main:   `<main>本文<template><main>内部の雛形</main></template></main>`,
		},
		{
			name:   "SVGとMathMLとCDATA内のmainは対象外",
			prefix: `<svg><![CDATA[</svg><main>偽の本文</main>]]><main>SVG</main></svg><math><main>MathML</main></math><svg/>`,
			main:   `<main>本文<svg><main>SVG内</main></svg></main>`,
		},
		{
			name:   "最初のmainだけを抽出",
			prefix: "\n<header>前置き</header>\n",
			main:   `<main>最初</main>`,
			suffix: `<main>二つ目</main>`,
		},
		{
			name:   "Tokenizerの読み込み境界をまたぐ本文",
			prefix: strings.Repeat("前置き\r\n", 2000),
			main:   "<main>\r\n" + strings.Repeat("本文&amp;\r\n", 2000) + "</main>",
			suffix: strings.Repeat("後書き\r\n", 2000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := []byte(tt.prefix + tt.main + tt.suffix)
			original := bytes.Clone(source)
			got, err := extractMainHTML(source)
			if err != nil {
				t.Fatalf("extractMainHTML() error = %v", err)
			}
			if !bytes.Equal(got, []byte(tt.main)) {
				t.Fatalf("抽出したmainのバイト列が元の範囲と一致しません: got %q, want %q", got, tt.main)
			}
			if !bytes.Equal(source, original) {
				t.Fatal("入力のバイト列が変更されています")
			}
		})
	}
}

func TestExtractMainHTML_RejectsMissingOrInvalidMain(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "空の入力"},
		{name: "mainなし", source: `<html><body><div>本文</div></body></html>`},
		{name: "コメント内だけ", source: `<!-- <main>本文</main> -->`},
		{name: "script内だけ", source: `<script>"<main>本文</main>"</script>`},
		{name: "テンプレート内だけ", source: `<template><main>本文</main></template>`},
		{name: "名前の前方一致を避ける", source: `<main-content>本文</main-content>`},
		{name: "終了タグなし", source: `<main>本文</body></html>`},
		{name: "終了タグが未完", source: `<main>本文</main`},
		{name: "script内の終了タグだけ", source: `<main><script>"</main>"</script>`},
		{name: "開始タグなし", source: `</main><main>本文</main>`},
		{name: "入れ子のmain", source: `<main><main>本文</main></main>`},
		{name: "自己終了のmain", source: `<main/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMainHTML([]byte(tt.source))
			if err == nil || got != nil {
				t.Fatalf("extractMainHTML() = %q, %v; 不正な範囲を返さずエラーになる必要があります", got, err)
			}
		})
	}
}
