package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type ScrapeResult struct {
	PageType       string        `json:"page_type"`
	Title          string        `json:"title"`
	Author         string        `json:"author"`
	RawHTML        []string      `json:"raw_html"`
	TextContent    []string      `json:"text_content"`
	FullPageHTML   string        `json:"full_page_html"`
	IndexPagesHTML []string      `json:"index_pages_html"`
	Chapters       []ChapterInfo `json:"chapters,omitempty"`
	Error          string        `json:"error,omitempty"`
}

type ChapterInfo struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	Content      string `json:"content"`
	RawHTML      string `json:"raw_html"`
	FullPageHTML string `json:"full_page_html"`
	RetryCount   int    `json:"retry_count"`
	Failed       bool   `json:"failed"`
}

// StartScraping はWailsのバインディングとして公開される関数です
func (a *App) StartScraping(url string) ScrapeResult {
	result := ScrapeResult{}

	// HTTPクライアントの設定
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	// リクエストの作成
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("リクエスト作成エラー: %v\n", err)
		result.Error = err.Error()
		return result
	}

	// ヘッダーの設定
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")

	// ノクターンノベルズの年齢確認用Cookie
	if strings.Contains(url, "novel18.syosetu.com") {
		req.Header.Set("Cookie", "over18=yes")
	}

	// ノクターンノベルズの年齢確認用Cookie
	if strings.Contains(url, "novel18.syosetu.com") {
		req.Header.Set("Cookie", "over18=yes")
	}

	// HTTPリクエストの実行
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("リクエストエラー: %v\n", err)
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	// HTMLの解析
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// タイトルの取得
	result.Title = doc.Find("h1").Text()

	// 作者名の取得
	result.Author = doc.Find(".p-novel__author a").Text()
	if result.Author == "" {
		// フォールバック：異なるセレクタを試す
		result.Author = doc.Find(".p-novel__author").Text()
		if result.Author == "" {
			result.Author = "不明な作者"
		}
	}

	// ページタイプの判定（連載か短編か）
	// エピソードリストの存在をチェック
	if doc.Find(".p-eplist").Length() > 0 || doc.Find(".p-eplist__sublist").Length() > 0 {
		result.PageType = "rensai" // 連載
	} else if doc.Find(".p-novel__body").Length() > 0 {
		result.PageType = "short" // 短編
	} else {
		result.Error = "不明なページタイプです"
		return result
	}

	// ページタイプに応じた処理
	switch result.PageType {
	case "rensai":
		// 連載の場合、エピソードリストを取得
		if err := a.scrapeChapterList(&result, doc, url); err != nil {
			result.Error = err.Error()
			return result
		}
	case "short":
		// 短編の場合、本文を直接取得
		content, err := a.extractContent(doc)
		if err != nil {
			result.Error = err.Error()
			return result
		}

		// テキストコンテンツを保存（TXTファイル用）
		result.TextContent = append(result.TextContent, content)

		// HTML構造も取得（HTMLファイル用）
		rawHTML, err := a.extractRawHTML(doc)
		if err != nil {
			log.Printf("HTML構造の取得に失敗しました: %v", err)
			// HTML構造取得に失敗した場合は空文字列を追加
			result.RawHTML = append(result.RawHTML, "")
		} else {
			result.RawHTML = append(result.RawHTML, rawHTML)
		}

		// ページ全体のHTMLも取得（短編用）
		fullPageHTML, err := a.extractFullPageHTML(doc, url)
		if err != nil {
			log.Printf("ページ全体のHTML取得に失敗しました: %v", err)
		} else {
			// 短編の場合、結果に追加
			result.FullPageHTML = fullPageHTML
			log.Printf("ページ全体のHTMLを取得しました（%d文字）", len(fullPageHTML))
		}
	}

	return result
}

// scrapeChapterList は連載小説のエピソードリストを取得します
func (a *App) scrapeChapterList(result *ScrapeResult, doc *goquery.Document, baseURL string) error {
	// 最初のページから開始（既に取得済みのdocを使用）
	pageDoc := doc

	// 最初のページのHTMLを保存
	firstPageHTML, err := a.extractFullPageHTML(pageDoc, baseURL)
	if err != nil {
		log.Printf("最初のページのHTML取得に失敗しました: %v", err)
	} else {
		result.IndexPagesHTML = append(result.IndexPagesHTML, firstPageHTML)
	}

	for {
		// エピソードリストを取得
		pageDoc.Find(".p-eplist__sublist a").Each(func(i int, s *goquery.Selection) {
			chapterURL, exists := s.Attr("href")
			if !exists {
				return
			}

			// 相対URLの場合は絶対URLに変換
			if !strings.HasPrefix(chapterURL, "http") {
				if strings.HasPrefix(chapterURL, "/") {
					if strings.Contains(baseURL, "novel18.syosetu.com") {
						chapterURL = "https://novel18.syosetu.com" + chapterURL
					} else {
						chapterURL = "https://ncode.syosetu.com" + chapterURL
					}
				} else {
					chapterURL = baseURL + "/" + chapterURL
				}
			}

			chapterTitle := strings.TrimSpace(s.Text())

			chapter := ChapterInfo{
				Title: chapterTitle,
				URL:   chapterURL,
			}

			result.Chapters = append(result.Chapters, chapter)
		})

		// 「次へ」ボタンを探す
		nextLink, exists := pageDoc.Find(".c-pager__item--next").Attr("href")
		if !exists || nextLink == "" {
			// 次のページがない場合は終了
			break
		}

		// 次のページのURLを作成
		var nextURL string
		if !strings.HasPrefix(nextLink, "http") {
			if strings.HasPrefix(nextLink, "/") {
				if strings.Contains(baseURL, "novel18.syosetu.com") {
					nextURL = "https://novel18.syosetu.com" + nextLink
				} else {
					nextURL = "https://ncode.syosetu.com" + nextLink
				}
			} else {
				// 相対パスの場合
				nextURL = baseURL + nextLink
			}
		} else {
			nextURL = nextLink
		}

		// 次のページを取得
		var err error
		pageDoc, err = a.fetchPage(nextURL)
		if err != nil {
			return fmt.Errorf("次のページの取得に失敗しました: %w", err)
		}

		// 次のページのHTMLも保存
		pageHTML, err := a.extractFullPageHTML(pageDoc, nextURL)
		if err != nil {
			log.Printf("ページのHTML取得に失敗しました: %v", err)
		} else {
			result.IndexPagesHTML = append(result.IndexPagesHTML, pageHTML)
		}
	}

	if len(result.Chapters) == 0 {
		return fmt.Errorf("エピソードリストを取得できませんでした")
	}

	return nil
}

// fetchPage はURLからHTMLドキュメントを取得します
func (a *App) fetchPage(url string) (*goquery.Document, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")

	// ノクターンノベルズの年齢確認用Cookie
	if strings.Contains(url, "novel18.syosetu.com") {
		req.Header.Set("Cookie", "over18=yes")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// extractContent はHTMLドキュメントから本文を抽出します
func (a *App) extractContent(doc *goquery.Document) (string, error) {
	var contentParts []string

	// 小説家になろうの現在の構造に対応
	// p-novel__body 内の p-novel__text を取得
	doc.Find(".p-novel__body .p-novel__text").Each(func(i int, s *goquery.Selection) {
		// HTMLを取得してルビ変換処理を適用
		html, err := s.Html()
		if err != nil {
			// HTMLが取得できない場合はテキストのみ取得
			text := strings.TrimSpace(s.Text())
			if text != "" {
				contentParts = append(contentParts, text)
			}
			return
		}

		// ルビ変換処理を適用
		convertedHTML := a.convertRubyToAozora(html)

		// HTMLタグを除去してテキストのみ抽出
		cleanText := a.removeHTMLTags(convertedHTML)
		cleanText = strings.TrimSpace(cleanText)

		if cleanText != "" {
			contentParts = append(contentParts, cleanText)
		}
	})

	if len(contentParts) > 0 {
		result := strings.Join(contentParts, "\n************************************************\n")
		log.Printf("本文を取得しました（%d部分）", len(contentParts))
		return result, nil
	}

	return "", fmt.Errorf("本文を取得できませんでした")
}

// removeHTMLTags はHTMLタグを除去してテキストのみを返します
func (a *App) removeHTMLTags(html string) string {
	// HTMLタグを除去する正規表現
	htmlTagPattern := regexp.MustCompile(`<[^>]*>`)
	text := htmlTagPattern.ReplaceAllString(html, "")

	// HTMLエンティティをデコード
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return text
}

// ScrapeChapter は個別のエピソードの内容を取得します（リトライ機能付き）
func (a *App) ScrapeChapter(chapterURL string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			log.Printf("Chapterの取得をリトライします（%d/%d回目）: %s", retry+1, maxRetries, chapterURL)
			// リトライ前に少し待機
			time.Sleep(time.Duration(retry) * time.Second)
		}

		content, err := a.scrapeChapterOnce(chapterURL)
		if err == nil {
			if retry > 0 {
				log.Printf("Chapterの取得に成功しました（%d回目で成功）: %s", retry+1, chapterURL)
			}
			return content, nil
		}

		lastErr = err
		log.Printf("Chapterの取得に失敗しました（%d/%d回目）: %s - エラー: %v", retry+1, maxRetries, chapterURL, err)
	}

	return "", fmt.Errorf("Chapterの取得に%d回失敗しました: %s - 最後のエラー: %w", maxRetries, chapterURL, lastErr)
}

// scrapeChapterOnce は個別のエピソードの内容を1回だけ取得します
func (a *App) scrapeChapterOnce(chapterURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", chapterURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")

	// ノクターンノベルズの年齢確認用Cookie
	if strings.Contains(chapterURL, "novel18.syosetu.com") {
		req.Header.Set("Cookie", "over18=yes")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// 共通のコンテンツ抽出関数を使用
	return a.extractContent(doc)
}

// ScrapeChapterWithHTML は個別のエピソードの内容とHTML構造を取得します（リトライ機能付き）
func (a *App) ScrapeChapterWithHTML(chapterURL string) (string, string, string, error) {
	const maxRetries = 3
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			log.Printf("ChapterのHTML取得をリトライします（%d/%d回目）: %s", retry+1, maxRetries, chapterURL)
			// リトライ前に少し待機
			time.Sleep(time.Duration(retry) * time.Second)
		}

		content, rawHTML, fullPageHTML, err := a.scrapeChapterWithHTMLOnce(chapterURL)
		if err == nil {
			if retry > 0 {
				log.Printf("ChapterのHTML取得に成功しました（%d回目で成功）: %s", retry+1, chapterURL)
			}
			return content, rawHTML, fullPageHTML, nil
		}

		lastErr = err
		log.Printf("ChapterのHTML取得に失敗しました（%d/%d回目）: %s - エラー: %v", retry+1, maxRetries, chapterURL, err)
	}

	return "", "", "", fmt.Errorf("ChapterのHTML取得に%d回失敗しました: %s - 最後のエラー: %w", maxRetries, chapterURL, lastErr)
}

// scrapeChapterWithHTMLOnce は個別のエピソードの内容とHTML構造を1回だけ取得します
func (a *App) scrapeChapterWithHTMLOnce(chapterURL string) (string, string, string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	req, err := http.NewRequest("GET", chapterURL, nil)
	if err != nil {
		return "", "", "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36")

	// ノクターンノベルズの年齢確認用Cookie
	if strings.Contains(chapterURL, "novel18.syosetu.com") {
		req.Header.Set("Cookie", "over18=yes")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	// テキストコンテンツを取得
	content, err := a.extractContent(doc)
	if err != nil {
		return "", "", "", err
	}

	// HTML構造を取得
	rawHTML, err := a.extractRawHTML(doc)
	if err != nil {
		return content, "", "", err
	}

	// ページ全体のHTMLを取得
	fullPageHTML, err := a.extractFullPageHTML(doc, chapterURL)
	if err != nil {
		return content, rawHTML, "", err
	}

	return content, rawHTML, fullPageHTML, nil
}

// extractRawHTML は元のHTML構造を取得します
func (a *App) extractRawHTML(doc *goquery.Document) (string, error) {
	// 小説本文部分のHTMLを取得（.p-novel__body内のすべて）
	novelBody := doc.Find(".p-novel__body")
	if novelBody.Length() == 0 {
		return "", fmt.Errorf("小説本文が見つかりませんでした")
	}

	html, err := novelBody.Html()
	if err != nil {
		return "", fmt.Errorf("HTML取得エラー: %w", err)
	}

	return html, nil
}

// extractFullPageHTML はページ全体のHTMLを取得します
func (a *App) extractFullPageHTML(doc *goquery.Document, originalURL string) (string, error) {
	// ページ全体のHTMLを取得
	html, err := doc.Html()
	if err != nil {
		return "", fmt.Errorf("ページ全体のHTML取得エラー: %w", err)
	}

	// URLから適切なベースURLを決定
	var baseURL string
	if strings.Contains(originalURL, "novel18.syosetu.com") {
		baseURL = "https://novel18.syosetu.com"
	} else {
		baseURL = "https://ncode.syosetu.com"
	}

	// 相対パスを絶対パスに変換
	html = a.convertRelativeToAbsolutePaths(html, baseURL)

	return html, nil
}

// convertRelativeToAbsolutePaths は相対パスを絶対パスに変換します
func (a *App) convertRelativeToAbsolutePaths(html string, baseURL string) string {

	// CSS ファイルのパスを変換
	cssPattern := regexp.MustCompile(`href="(/[^"]*\.css[^"]*)"`)
	html = cssPattern.ReplaceAllStringFunc(html, func(match string) string {
		path := cssPattern.FindStringSubmatch(match)[1]
		return fmt.Sprintf(`href="%s%s"`, baseURL, path)
	})

	// JavaScript ファイルのパスを変換
	jsPattern := regexp.MustCompile(`src="(/[^"]*\.js[^"]*)"`)
	html = jsPattern.ReplaceAllStringFunc(html, func(match string) string {
		path := jsPattern.FindStringSubmatch(match)[1]
		return fmt.Sprintf(`src="%s%s"`, baseURL, path)
	})

	// 画像ファイルのパスを変換
	imgPattern := regexp.MustCompile(`src="(/[^"]*\.(png|jpg|jpeg|gif|svg|webp)[^"]*)"`)
	html = imgPattern.ReplaceAllStringFunc(html, func(match string) string {
		path := imgPattern.FindStringSubmatch(match)[1]
		return fmt.Sprintf(`src="%s%s"`, baseURL, path)
	})

	// その他のリンクを変換
	linkPattern := regexp.MustCompile(`href="(/[^"]*)"`)
	html = linkPattern.ReplaceAllStringFunc(html, func(match string) string {
		path := linkPattern.FindStringSubmatch(match)[1]
		// 外部サイトへのリンクやanchor linkは変換しない
		if strings.HasPrefix(path, "#") || strings.Contains(path, "://") {
			return match
		}
		return fmt.Sprintf(`href="%s%s"`, baseURL, path)
	})

	return html
}
