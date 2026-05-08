package main

import "testing"

func TestApp_FormatAozoraRuby_EmphasisDot(t *testing.T) {
	app := NewApp()

	result := app.formatAozoraRuby("重要", "・・", "")
	expected := "［＃傍点］重要［＃傍点終わり］"
	if result != expected {
		t.Fatalf("formatAozoraRuby() = %q, want %q", result, expected)
	}
}

func TestApp_FormatChapterContent_ReadableText(t *testing.T) {
	app := NewApp()
	content := "これは<ruby>漢字<rt>かんじ</rt></ruby>と<ruby>重要<rt>・・</rt></ruby>です。"

	result := app.formatChapterContent("小説タイトル", "作者", "第一話", content, textFormatReadable)
	expected := "第一話\n\nこれは漢字（かんじ）と重要です。\n"
	if result != expected {
		t.Fatalf("formatChapterContent() = %q, want %q", result, expected)
	}
}

func TestApp_ConvertAozoraToReadableText(t *testing.T) {
	app := NewApp()
	input := "｜abc《えーびーしー》と漢字《かんじ》と［＃傍点］重要［＃傍点終わり］［＃太字］強調［＃太字終わり］"

	result := app.convertAozoraToReadableText(input)
	expected := "abc（えーびーしー）と漢字（かんじ）と重要強調"
	if result != expected {
		t.Fatalf("convertAozoraToReadableText() = %q, want %q", result, expected)
	}
}
