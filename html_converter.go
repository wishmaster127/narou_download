package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// HTMLConverter は HTML を青空文庫形式に変換するための構造体
type HTMLConverter struct {
	text                string
	stripDecorationTag  bool
	illustCurrentURL    string
	illustGrepPattern   *regexp.Regexp
}

// NewHTMLConverter は新しい HTMLConverter インスタンスを作成
func NewHTMLConverter(text string) *HTMLConverter {
	return &HTMLConverter{
		text:               text,
		stripDecorationTag: false,
		illustGrepPattern:  regexp.MustCompile(`<img.+?src="(?P<src>.+?)".*?>`),
	}
}

// SetIllustSetting は挿絵置換のための設定を変更
func (h *HTMLConverter) SetIllustSetting(currentURL string, grepPattern string) error {
	h.illustCurrentURL = currentURL
	if grepPattern != "" {
		pattern, err := regexp.Compile(grepPattern)
		if err != nil {
			return fmt.Errorf("invalid grep pattern: %w", err)
		}
		h.illustGrepPattern = pattern
	}
	return nil
}

// SetStripDecorationTag は装飾タグを削除するかどうかを設定
func (h *HTMLConverter) SetStripDecorationTag(strip bool) {
	h.stripDecorationTag = strip
}

// ToAozora は HTML を青空文庫形式に変換
func (h *HTMLConverter) ToAozora(preHTML bool) string {
	text := h.text

	// 改行変換（preHTML が false の場合のみ）
	if !preHTML {
		text = h.brToAozora(text)
	}

	// 段落変換
	text = h.pToAozora(text)

	// ルビ変換
	text = h.rubyToAozora(text)

	// 装飾タグ変換（stripDecorationTag が無効の場合のみ）
	if !h.stripDecorationTag {
		text = h.bToAozora(text)
		text = h.iToAozora(text)
		text = h.sToAozora(text)
	}

	// 挿絵変換
	text = h.imgToAozora(text)

	// 強調点変換
	text = h.emToSesame(text)

	// HTMLタグ削除
	text = h.deleteTag(text)

	// HTMLエンティティ復元
	text = restoreHTMLEntity(text)

	return text
}

// brToAozora は <br> タグを改行文字に変換
func (h *HTMLConverter) brToAozora(text string) string {
	// 既存の改行文字を削除
	re1 := regexp.MustCompile(`[\r\n]+`)
	text = re1.ReplaceAllString(text, "")
	
	// <br> タグを改行に変換
	re2 := regexp.MustCompile(`<br.*?>`)
	return re2.ReplaceAllString(text, "\n")
}

// pToAozora は </p> タグを改行文字に変換
func (h *HTMLConverter) pToAozora(text string) string {
	re := regexp.MustCompile(`(?i)\n?</p>`)
	return re.ReplaceAllString(text, "\n")
}

// rubyToAozora は ruby タグを青空文庫形式に変換
func (h *HTMLConverter) rubyToAozora(text string) string {
	// 《》を≪≫に変換
	text = strings.ReplaceAll(text, "《", "≪")
	text = strings.ReplaceAll(text, "》", "≫")

	// <ruby> タグを青空文庫形式に変換
	re := regexp.MustCompile(`(?i)<ruby>(.+?)</ruby>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		// ruby タグの内容を取得
		rubyContent := re.FindStringSubmatch(match)[1]
		
		// <rt> で分割
		rtRe := regexp.MustCompile(`(?i)<rt>`)
		parts := rtRe.Split(rubyContent, 2)
		
		if len(parts) < 2 {
			// rt タグがない場合はタグを削除して返す
			return h.deleteTag(parts[0])
		}

		// ruby base（漢字部分）の取得
		rpRe := regexp.MustCompile(`(?i)<rp>`)
		baseParts := rpRe.Split(parts[0], 2)
		rubyBase := h.deleteTag(baseParts[0])

		// ruby text（ふりがな部分）の取得
		textParts := rpRe.Split(parts[1], 2)
		rubyText := h.deleteTag(textParts[0])

		return fmt.Sprintf("｜%s《%s》", rubyBase, rubyText)
	})
}

// bToAozora は <b> タグを青空文庫形式に変換
func (h *HTMLConverter) bToAozora(text string) string {
	re1 := regexp.MustCompile(`(?i)<b>`)
	text = re1.ReplaceAllString(text, "［＃太字］")
	
	re2 := regexp.MustCompile(`(?i)</b>`)
	return re2.ReplaceAllString(text, "［＃太字終わり］")
}

// iToAozora は <i> タグを青空文庫形式に変換
func (h *HTMLConverter) iToAozora(text string) string {
	re1 := regexp.MustCompile(`(?i)<i>`)
	text = re1.ReplaceAllString(text, "［＃斜体］")
	
	re2 := regexp.MustCompile(`(?i)</i>`)
	return re2.ReplaceAllString(text, "［＃斜体終わり］")
}

// sToAozora は <s> タグを青空文庫形式に変換
func (h *HTMLConverter) sToAozora(text string) string {
	re1 := regexp.MustCompile(`(?i)<s>`)
	text = re1.ReplaceAllString(text, "［＃取消線］")
	
	re2 := regexp.MustCompile(`(?i)</s>`)
	return re2.ReplaceAllString(text, "［＃取消線終わり］")
}

// imgToAozora は img タグを青空文庫形式に変換
func (h *HTMLConverter) imgToAozora(text string) string {
	if h.illustGrepPattern == nil {
		return text
	}

	return h.illustGrepPattern.ReplaceAllStringFunc(text, func(match string) string {
		// src属性を抽出
		matches := h.illustGrepPattern.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		
		src := matches[1]
		
		// 相対URLの場合は絶対URLに変換
		if h.illustCurrentURL != "" {
			if baseURL, err := url.Parse(h.illustCurrentURL); err == nil {
				if imgURL, err := baseURL.Parse(src); err == nil {
					src = imgURL.String()
				}
			}
		}
		
		return fmt.Sprintf("［＃挿絵（%s）入る］", src)
	})
}

// emToSesame は強調点用の em タグを青空文庫形式に変換
func (h *HTMLConverter) emToSesame(text string) string {
	re := regexp.MustCompile(`<em class="emphasisDots">(.+?)</em>`)
	return re.ReplaceAllString(text, "［＃傍点］$1［＃傍点終わり］")
}

// deleteTag は HTML タグを削除
func (h *HTMLConverter) deleteTag(text string) string {
	re := regexp.MustCompile(`<.+?>`)
	return re.ReplaceAllString(text, "")
}

// restoreHTMLEntity は HTML エンティティを復元
func restoreHTMLEntity(text string) string {
	replacements := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   `"`,
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&#39;":    "'",
		"&#34;":    `"`,
		"&hellip;": "…",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
		"&ldquo;":  "\u201c",
		"&rdquo;":  "\u201d",
	}

	result := text
	for entity, char := range replacements {
		result = strings.ReplaceAll(result, entity, char)
	}

	// 数値文字参照 (&#数値; 形式) の処理
	re := regexp.MustCompile(`&#(\d+);`)
	result = re.ReplaceAllStringFunc(result, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		
		var code int
		if _, err := fmt.Sscanf(matches[1], "%d", &code); err != nil {
			return match
		}
		
		return string(rune(code))
	})

	// 16進数文字参照 (&#x16進数; 形式) の処理
	reHex := regexp.MustCompile(`&#x([0-9a-fA-F]+);`)
	result = reHex.ReplaceAllStringFunc(result, func(match string) string {
		matches := reHex.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}
		
		var code int
		if _, err := fmt.Sscanf(matches[1], "%x", &code); err != nil {
			return match
		}
		
		return string(rune(code))
	})

	return result
}