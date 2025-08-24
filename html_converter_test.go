package main

import (
	"testing"
)

func TestHTMLConverter_BrToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的なbrタグ変換",
			input:    "これは<br>テストです<br />",
			expected: "これは\nテストです\n",
		},
		{
			name:     "改行文字を含むHTML",
			input:    "これは\n<br>\nテストです\r\n<br>",
			expected: "これは\nテストです\n",
		},
		{
			name:     "brタグなし",
			input:    "これは普通のテキストです",
			expected: "これは普通のテキストです",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.brToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("brToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_RubyToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的なrubyタグ変換",
			input:    "<ruby>漢字<rt>かんじ</rt></ruby>",
			expected: "｜漢字《かんじ》",
		},
		{
			name:     "rpタグ付きrubyタグ",
			input:    "<ruby><rb>漢字</rb><rp>（</rp><rt>かんじ</rt><rp>）</rp></ruby>",
			expected: "｜漢字《かんじ》",
		},
		{
			name:     "rtタグがない場合",
			input:    "<ruby>漢字</ruby>",
			expected: "漢字",
		},
		{
			name:     "複数のrubyタグ",
			input:    "<ruby>漢字<rt>かんじ</rt></ruby>と<ruby>仮名<rt>かな</rt></ruby>",
			expected: "｜漢字《かんじ》と｜仮名《かな》",
		},
		{
			name:     "《》の変換確認",
			input:    "これは《テスト》です",
			expected: "これは≪テスト≫です",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.rubyToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("rubyToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_BToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的な太字変換",
			input:    "これは<b>太字</b>です",
			expected: "これは［＃太字］太字［＃太字終わり］です",
		},
		{
			name:     "大文字小文字混在",
			input:    "<B>太字1</B>と<b>太字2</b>",
			expected: "［＃太字］太字1［＃太字終わり］と［＃太字］太字2［＃太字終わり］",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.bToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("bToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_IToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的な斜体変換",
			input:    "これは<i>斜体</i>です",
			expected: "これは［＃斜体］斜体［＃斜体終わり］です",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.iToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("iToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_SToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的な取消線変換",
			input:    "これは<s>取消線</s>です",
			expected: "これは［＃取消線］取消線［＃取消線終わり］です",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.sToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("sToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_ImgToAozora(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		currentURL  string
		expected    string
	}{
		{
			name:       "基本的な画像変換",
			input:      `<img src="test.jpg" alt="テスト画像">`,
			currentURL: "",
			expected:   "［＃挿絵（test.jpg）入る］",
		},
		{
			name:       "相対URLの変換",
			input:      `<img src="images/test.jpg">`,
			currentURL: "https://example.com/novel/",
			expected:   "［＃挿絵（https://example.com/novel/images/test.jpg）入る］",
		},
		{
			name:       "複数の画像",
			input:      `<img src="img1.jpg"><img src="img2.png">`,
			currentURL: "",
			expected:   "［＃挿絵（img1.jpg）入る］［＃挿絵（img2.png）入る］",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			if tt.currentURL != "" {
				converter.SetIllustSetting(tt.currentURL, "")
			}
			result := converter.imgToAozora(tt.input)
			if result != tt.expected {
				t.Errorf("imgToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_EmToSesame(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的な傍点変換",
			input:    `<em class="emphasisDots">重要</em>`,
			expected: "［＃傍点］重要［＃傍点終わり］",
		},
		{
			name:     "複数の傍点",
			input:    `<em class="emphasisDots">重要1</em>と<em class="emphasisDots">重要2</em>`,
			expected: "［＃傍点］重要1［＃傍点終わり］と［＃傍点］重要2［＃傍点終わり］",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.emToSesame(tt.input)
			if result != tt.expected {
				t.Errorf("emToSesame() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_DeleteTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的なタグ削除",
			input:    "<p>これは<span>テスト</span>です</p>",
			expected: "これはテストです",
		},
		{
			name:     "複雑なタグ削除",
			input:    `<div class="content"><p style="color:red;">テスト</p></div>`,
			expected: "テスト",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			result := converter.deleteTag(tt.input)
			if result != tt.expected {
				t.Errorf("deleteTag() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRestoreHTMLEntity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本的なエンティティ",
			input:    "&amp;&lt;&gt;&quot;&apos;",
			expected: "&<>\"'",
		},
		{
			name:     "数値文字参照",
			input:    "&#65;&#66;&#67;",
			expected: "ABC",
		},
		{
			name:     "16進数文字参照",
			input:    "&#x41;&#x42;&#x43;",
			expected: "ABC",
		},
		{
			name:     "日本語文字",
			input:    "これは&quot;テスト&quot;です&amp;確認",
			expected: "これは\"テスト\"です&確認",
		},
		{
			name:     "特殊記号",
			input:    "&hellip;&mdash;&ndash;",
			expected: "…—–",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := restoreHTMLEntity(tt.input)
			if result != tt.expected {
				t.Errorf("restoreHTMLEntity() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHTMLConverter_ToAozora(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		preHTML  bool
	}{
		{
			name: "総合的な変換テスト",
			input: `<p>これは<ruby>漢字<rt>かんじ</rt></ruby>です。</p>
<p><b>太字</b>と<i>斜体</i>があります。</p>
<img src="illust.jpg" alt="挿絵">
<p><em class="emphasisDots">傍点付きテキスト</em></p>`,
			expected: `これは｜漢字《かんじ》です。
［＃太字］太字［＃太字終わり］と［＃斜体］斜体［＃斜体終わり］があります。
［＃挿絵（illust.jpg）入る］［＃傍点］傍点付きテキスト［＃傍点終わり］
`,
			preHTML: false,
		},
		{
			name: "HTMLエンティティを含む変換",
			input: `<p>&quot;Hello&quot; &amp; &lt;World&gt;</p>
<ruby>漢字<rt>かんじ</rt></ruby>`,
			expected: `"Hello" & <World>
｜漢字《かんじ》`,
			preHTML: false,
		},
		{
			name: "装飾タグ削除モード",
			input: `<p><b>太字</b>と<i>斜体</i></p>`,
			expected: `太字と斜体
`,
			preHTML: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := NewHTMLConverter(tt.input)
			if tt.name == "装飾タグ削除モード" {
				converter.SetStripDecorationTag(true)
			}
			result := converter.ToAozora(tt.preHTML)
			if result != tt.expected {
				t.Errorf("ToAozora() = %q, want %q", result, tt.expected)
			}
		})
	}
}