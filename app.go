package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// App struct
type App struct {
	ctx      context.Context
	settings Settings
}

// Settings はアプリケーションの設定を表す構造体
type Settings struct {
	URL            string `json:"url"`
	SavePath       string `json:"savePath"`
	Encoding       string `json:"encoding"`
	LineEnding     string `json:"lineEnding"`
	CreateHtml     bool   `json:"createHtml"`
	CreateTxt      bool   `json:"createTxt"`
	CreateCombined bool   `json:"createCombined"`
	ShowInFront    bool   `json:"showInFront"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// setupSavePath は保存先のパスを設定します
func (a *App) setupSavePath(savePath string, title string) (string, error) {
	if savePath == "" {
		// 実行ファイルのディレクトリを取得
		exePath, err := os.Executable()
		if err != nil {
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("実行ファイルのパスを取得できませんでした: %v", err))
			return "", fmt.Errorf("実行ファイルのパスを取得できませんでした: %w", err)
		}
		exeDir := filepath.Dir(exePath)

		// 小説のタイトルと同じ名前のディレクトリを作成
		savePath = filepath.Join(exeDir, title)
	}

	// ディレクトリを作成
	if err := os.MkdirAll(savePath, 0755); err != nil {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("保存先ディレクトリの作成に失敗しました: %v", err))
		return "", fmt.Errorf("保存先ディレクトリの作成に失敗しました: %w", err)
	}
	runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("保存先ディレクトリを作成しました: %s", savePath))

	return savePath, nil
}

// DownloadNovel は小説のダウンロードを開始します
func (a *App) DownloadNovel(url string, savePath string, options map[string]interface{}) error {
	// 進捗状況を更新
	runtime.EventsEmit(a.ctx, "progress", 0)
	runtime.EventsEmit(a.ctx, "log", "HTMLの取得を開始します...")

	// 各話URLの場合は小説インデックスURLに変換
	processedURL := a.convertToIndexURL(url)
	if processedURL != url {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("各話URLを検出しました。小説全体をダウンロードします: %s", processedURL))
	}

	// スクレイピングの実行
	result := a.StartScraping(processedURL)
	if result.Error != "" {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("スクレイピングエラー: %s", result.Error))
		return fmt.Errorf("スクレイピングエラー: %s", result.Error)
	}

	// 保存先の設定
	savePath, err := a.setupSavePath(savePath, result.Title)
	if err != nil {
		return err
	}

	// 設定の取得
	encoding := options["encoding"].(string)
	lineEnding := options["lineEnding"].(string)
	createHtml := options["createHtml"].(bool)
	createTxt := options["createTxt"].(bool)
	createCombined := options["createCombined"].(bool)

	// 連載か短編かで処理を分岐
	switch result.PageType {
	case "rensai":
		return a.downloadRensai(savePath, result, createHtml, createTxt, encoding, lineEnding, createCombined)
	case "short":
		return a.downloadShort(savePath, result, createHtml, createTxt, encoding, lineEnding, url)
	default:
		return fmt.Errorf("不明なページタイプ: %s", result.PageType)
	}
}

// downloadRensai は連載小説のダウンロード処理を行います（リトライ機能付き）
func (a *App) downloadRensai(savePath string, result ScrapeResult, createHtml, createTxt bool, encoding, lineEnding string, createCombined bool) error {
	if len(result.Chapters) == 0 {
		return fmt.Errorf("エピソードが見つかりませんでした")
	}

	totalChapters := len(result.Chapters)
	runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話を取得しました。ダウンロードを開始します...", totalChapters))
	runtime.EventsEmit(a.ctx, "progressText", fmt.Sprintf("0/%d話", totalChapters))

	// HTMLファイル用のディレクトリ作成は無効化
	// var htmlDir string
	// if createHtml {
	// 	htmlDir = filepath.Join(savePath, "html")
	// 	if err := os.MkdirAll(htmlDir, 0755); err != nil {
	// 		return fmt.Errorf("htmlディレクトリの作成に失敗しました: %w", err)
	// 	}
	// }

	// エピソード別コンテンツの取得
	var allChapterContents []string
	novelCode := extractNovelCodeFromURL(result.Chapters[0].URL) // 最初のエピソードURLから小説番号を取得
	var failedChapters int
	const maxFailures = 3

	for i, chapter := range result.Chapters {
		runtime.EventsEmit(a.ctx, "progress", int(float64(i)/float64(totalChapters)*80)) // 80%までエピソード取得用
		runtime.EventsEmit(a.ctx, "progressText", fmt.Sprintf("%d/%d話", i, totalChapters))

		// ファイル名を先に生成してスキップチェック
		episodeNumber := extractEpisodeNumberFromURL(chapter.URL)
		// エピソード番号が取得できない場合や、全話が"1"になってしまう場合は、インデックス番号を使用
		if episodeNumber == "" || (i > 0 && episodeNumber == "1") {
			episodeNumber = fmt.Sprintf("%d", i+1)
		}
		chapterFileName := generateFileName(novelCode, episodeNumber)

		// 既に保存済みかチェック
		if a.shouldSkipEpisode(savePath, chapterFileName, episodeNumber, createHtml, createTxt) {
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話: %s はすでに保存済みです。スキップします。", i+1, chapter.Title))
			continue
		}

		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話: %s を取得中...", i+1, chapter.Title))

		// Chapterの取得（リトライ機能付き）
		content, rawHTML, fullPageHTML, err := a.ScrapeChapterWithHTML(chapter.URL)
		if err != nil {
			failedChapters++
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話の取得に失敗しました: %v （失敗回数: %d/%d）", i+1, err, failedChapters, maxFailures))

			// 失敗回数が上限に達した場合は全体を停止
			if failedChapters >= maxFailures {
				return fmt.Errorf("Chapterの取得に%d回失敗したため、ダウンロードを停止します。最後のエラー: %v", maxFailures, err)
			}
			continue
		}

		// 取得に成功した場合は失敗カウンターをリセット
		failedChapters = 0

		result.Chapters[i].Content = content
		result.Chapters[i].RawHTML = rawHTML
		result.Chapters[i].FullPageHTML = fullPageHTML

		// 連結ファイル用に各話のフォーマットされたコンテンツを保存（タイトル・作者名なし）
		chapterContentForCombined := a.formatChapterContentForCombined(chapter.Title, content)
		allChapterContents = append(allChapterContents, chapterContentForCombined)

		// 連載の場合、次のエピソードまで10秒間隔を開ける（最後のエピソード以外）
		if i < len(result.Chapters)-1 {
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話取得完了。10秒待機中...", i+1))
			time.Sleep(10 * time.Second)
		}

		// ファイル保存（リトライ機能付き）
		if createTxt {
			// 各話のフォーマット（タイトル、作者名、話タイトル、本文）
			formattedContent := a.formatChapterContent(result.Title, result.Author, chapter.Title, content)
			if err := a.saveTextFileWithRetry(savePath, chapterFileName, formattedContent, encoding, lineEnding); err != nil {
				runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話の保存に失敗しました: %v", i+1, err))
			}
		}

		// HTMLファイル保存は無効化
		// if createHtml {
		// 	// 元ページ全体のHTMLからiframeを除去してから保存
		// 	cleanHTML := a.removeIframes(fullPageHTML)
		// 	episodeFilePath := filepath.Join(htmlDir, fmt.Sprintf("%s.html", episodeNumber))
		// 	if err := os.WriteFile(episodeFilePath, []byte(cleanHTML), 0644); err != nil {
		// 		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("%d話のHTML保存に失敗しました: %v", i+1, err))
		// 	}
		// }
	}

	// インデックスページの作成は無効化
	// if createHtml && len(result.Chapters) > 0 {
	// 	runtime.EventsEmit(a.ctx, "progress", 85)
	// 	runtime.EventsEmit(a.ctx, "progressText", "インデックスページ作成中")
	// 	runtime.EventsEmit(a.ctx, "log", "インデックスページを作成中...")

	// 	if err := a.saveOriginalIndexPages(savePath, result.IndexPagesHTML, result.Chapters); err != nil {
	// 		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("インデックスページの作成に失敗しました: %v", err))
	// 	}
	// }

	// 連結ファイルの作成
	if createCombined && len(allChapterContents) > 0 {
		runtime.EventsEmit(a.ctx, "progress", 90)
		runtime.EventsEmit(a.ctx, "progressText", "連結ファイル作成中")
		runtime.EventsEmit(a.ctx, "log", "連結ファイルを作成中...")

		// 冒頭に小説タイトルと作者名を追加（ルビ変換済み）
		var combinedBuilder strings.Builder
		combinedBuilder.WriteString(a.convertRubyToAozora(result.Title))
		combinedBuilder.WriteString("\n")
		combinedBuilder.WriteString(a.convertRubyToAozora(result.Author))
		combinedBuilder.WriteString("\n\n\n")

		// 各話を点線区切りで連結
		combinedBuilder.WriteString(strings.Join(allChapterContents, "\n\n----------------\n\n\n"))

		combinedContent := combinedBuilder.String()

		if createTxt {
			if err := a.saveTextFileWithRetry(savePath, "all", combinedContent, encoding, lineEnding); err != nil {
				return fmt.Errorf("連結TXTファイルの保存に失敗しました: %w", err)
			}
		}
	}

	// 進捗状況を更新
	runtime.EventsEmit(a.ctx, "progress", 100)
	runtime.EventsEmit(a.ctx, "progressText", fmt.Sprintf("完了 (%d/%d話)", totalChapters, totalChapters))
	runtime.EventsEmit(a.ctx, "log", "ダウンロードが完了しました")

	return nil
}

// downloadShort は短編小説のダウンロード処理を行います
func (a *App) downloadShort(savePath string, result ScrapeResult, createHtml, createTxt bool, encoding, lineEnding string, originalURL string) error {
	runtime.EventsEmit(a.ctx, "progressText", "短編小説処理中")

	// 短編小説のファイル名生成（元のURLから小説番号を取得）
	novelCode := extractNovelCodeFromURL(originalURL)
	fileName := generateFileName(novelCode, "1") // 短編は常にエピソード1

	// 既に保存済みかチェック
	if a.shouldSkipEpisode(savePath, fileName, "1", createHtml, createTxt) {
		runtime.EventsEmit(a.ctx, "log", "短編小説はすでに保存済みです。スキップします。")
		runtime.EventsEmit(a.ctx, "progress", 100)
		runtime.EventsEmit(a.ctx, "progressText", "完了（スキップ）")
		return nil
	}

	// HTMLファイルの保存は無効化
	// if createHtml {
	// 	if result.FullPageHTML != "" {
	// 		// 元ページ全体のHTMLからiframeを除去してから保存
	// 		cleanHTML := a.removeIframes(result.FullPageHTML)
	// 		htmlFilePath := filepath.Join(savePath, fileName+".html")
	// 		if err := os.WriteFile(htmlFilePath, []byte(cleanHTML), 0644); err != nil {
	// 			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("HTMLファイルの保存に失敗しました: %v", err))
	// 			return fmt.Errorf("HTMLファイルの保存に失敗しました: %w", err)
	// 		}
	// 	} else {
	// 		// フォールバック：元のHTML生成方法（テキストコンテンツを使用）
	// 		htmlContent := a.generateShortNovelHTML(result.Title, strings.Join(result.TextContent, "\n"))
	// 		if err := a.saveHtmlFile(savePath, []string{htmlContent}, fileName); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	// テキストファイルの保存
	if createTxt {
		content := strings.Join(result.TextContent, "\n")
		if content == "" {
			runtime.EventsEmit(a.ctx, "log", "本文を取得できませんでした")
			return fmt.Errorf("本文を取得できませんでした")
		}

		// 短編小説のフォーマット（タイトル、作者名、話タイトルなし、本文）
		formattedContent := a.formatChapterContent(result.Title, result.Author, "", content)
		if err := a.saveTextFileWithRetry(savePath, fileName, formattedContent, encoding, lineEnding); err != nil {
			return err
		}
	}

	// 進捗状況を更新
	runtime.EventsEmit(a.ctx, "progress", 100)
	runtime.EventsEmit(a.ctx, "progressText", "完了")
	runtime.EventsEmit(a.ctx, "log", "ファイルの保存が完了しました")

	return nil
}

// saveHtmlFile はHTMLファイルを保存します
func (a *App) saveHtmlFile(savePath string, rawHTML []string, fileName string) error {
	htmlContent := strings.Join(rawHTML, "\n")
	filePath := filepath.Join(savePath, sanitizeFileName(fileName)+".html")
	err := os.WriteFile(filePath, []byte(htmlContent), 0644)
	if err != nil {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("HTMLファイルの保存に失敗しました: %v", err))
		return fmt.Errorf("HTMLファイルの保存に失敗しました: %w", err)
	}
	return nil
}

// saveTextFile はテキストファイルを保存します
func (a *App) saveTextFile(savePath, title, content, encoding, lineEnding string) error {
	// 改行コードの変換
	if lineEnding == "CR+LF" {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}

	// エンコードの変換
	var txtData []byte
	var err error
	switch encoding {
	case "UTF-8":
		txtData = []byte(content)
	case "UTF-16LE":
		// UTF-16LEエンコード
		utf16Data := make([]byte, 0, len(content)*2)
		for _, r := range content {
			utf16Data = append(utf16Data, byte(r), byte(r>>8))
		}
		txtData = utf16Data
	case "Shift-JIS":
		// Shift-JISエンコード
		encoder := japanese.ShiftJIS.NewEncoder()
		txtData, _, err = transform.Bytes(encoder, []byte(content))
		if err != nil {
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("Shift-JISエンコードエラー: %v", err))
			return fmt.Errorf("Shift-JISエンコードエラー: %w", err)
		}
	}

	// ファイルの保存
	baseFileName := filepath.Join(savePath, title)
	err = os.WriteFile(baseFileName+".txt", txtData, 0644)
	if err != nil {
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("TXTファイルの保存に失敗しました: %v", err))
		return fmt.Errorf("TXTファイルの保存に失敗しました: %w", err)
	}

	return nil
}

// saveTextFileWithRetry はテキストファイルの保存をリトライ機能付きで実行します
func (a *App) saveTextFileWithRetry(savePath, title, content, encoding, lineEnding string) error {
	const maxRetries = 3
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("ファイル保存をリトライします（%d/%d回目）: %s", retry+1, maxRetries, title))
			// リトライ前に少し待機
			time.Sleep(2 * time.Second)
		}

		err := a.saveTextFile(savePath, title, content, encoding, lineEnding)
		if err == nil {
			if retry > 0 {
				runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("ファイル保存に成功しました（%d回目で成功）: %s", retry+1, title))
			}
			return nil
		}

		lastErr = err
		runtime.EventsEmit(a.ctx, "log", fmt.Sprintf("ファイル保存に失敗しました（%d/%d回目）: %s - エラー: %v", retry+1, maxRetries, title, err))
	}

	return fmt.Errorf("ファイル保存に%d回失敗しました: %s - 最後のエラー: %w", maxRetries, title, lastErr)
}

// SelectFolder はフォルダ選択ダイアログを表示します
func (a *App) SelectFolder() (string, error) {
	// フォルダ選択ダイアログを表示
	selectedPath, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "保存先フォルダを選択",
	})
	if err != nil {
		return "", fmt.Errorf("エラーが発生しました: %w", err)
	}
	if selectedPath == "" {
		return "", fmt.Errorf("フォルダが選択されませんでした")
	}
	return selectedPath, nil
}

// OpenFolder は指定されたフォルダをOSのエクスプローラーで開きます
func (a *App) OpenFolder(path string) error {
	var cmd *exec.Cmd

	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("このOSには対応していません")
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("フォルダを開けませんでした: %w", err)
	}

	return nil
}

// 常に手前に表示
func (a *App) SetAlwaysOnTop(enable bool) {
	runtime.WindowSetAlwaysOnTop(a.ctx, enable)
}

// SaveSettings は設定をJSONファイルに保存します
func (a *App) SaveSettings(settings Settings) error {
	// 実行ファイルと同じディレクトリに設定ファイルを保存
	a.settings = settings
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("実行ファイルのパスを取得できませんでした: %w", err)
	}

	configPath := filepath.Join(filepath.Dir(exePath), "settings.json")
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("設定のJSON変換に失敗しました: %w", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("設定の保存に失敗しました: %w", err)
	}

	return nil
}

// LoadSettings はJSONファイルから設定を読み込みます
func (a *App) LoadSettings() (Settings, error) {
	var settings Settings

	// 実行ファイルと同じディレクトリから設定ファイルを読み込み
	exePath, err := os.Executable()
	if err != nil {
		return settings, fmt.Errorf("実行ファイルのパスを取得できませんでした: %w", err)
	}

	configPath := filepath.Join(filepath.Dir(exePath), "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 設定ファイルが存在しない場合はデフォルト値を返す
			return Settings{
				Encoding:   "UTF-8",
				LineEnding: "CR+LF",
				CreateHtml: true,
				CreateTxt:  true,
			}, nil
		}
		return settings, fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	err = json.Unmarshal(data, &settings)
	if err != nil {
		return settings, fmt.Errorf("設定のJSON解析に失敗しました: %w", err)
	}

	return settings, nil
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

func (a *App) shutdown(ctx context.Context) error {
	return a.SaveSettings(a.settings)
}

// sanitizeFileName はファイル名に使用できない文字を安全な文字に置換します
func sanitizeFileName(fileName string) string {
	// ファイル名に使用できない文字を置換
	fileName = strings.ReplaceAll(fileName, "/", "_")
	fileName = strings.ReplaceAll(fileName, "\\", "_")
	fileName = strings.ReplaceAll(fileName, ":", "_")
	fileName = strings.ReplaceAll(fileName, "*", "_")
	fileName = strings.ReplaceAll(fileName, "?", "_")
	fileName = strings.ReplaceAll(fileName, "\"", "_")
	fileName = strings.ReplaceAll(fileName, "<", "_")
	fileName = strings.ReplaceAll(fileName, ">", "_")
	fileName = strings.ReplaceAll(fileName, "|", "_")

	// 長すぎる場合は切り詰める
	if len(fileName) > 100 {
		fileName = fileName[:100]
	}

	return fileName
}

// extractNovelCodeFromURL はURLから小説番号を抽出します
func extractNovelCodeFromURL(url string) string {
	// URL例: https://ncode.syosetu.com/n3161kd/ または https://novel18.syosetu.com/n3161kd/1/
	// 正規表現で小説番号を抽出（nで始まり、英数字が続く）
	re := regexp.MustCompile(`/n([0-9]+[a-z]+)/`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return "N" + strings.ToUpper(matches[1])
	}

	// 上記で失敗した場合、従来の方法も試す
	parts := strings.Split(url, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "n") && len(part) > 1 {
			// 英数字のみをチェック
			isValid := true
			for i, char := range part {
				if i == 0 && char != 'n' {
					isValid = false
					break
				}
				if i > 0 && !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
					isValid = false
					break
				}
			}
			if isValid {
				return strings.ToUpper(part)
			}
		}
	}
	return "UNKNOWN"
}

// extractEpisodeNumberFromURL はURLからエピソード番号を抽出します
func extractEpisodeNumberFromURL(url string) string {
	// URL例: https://ncode.syosetu.com/n3161kd/1/ または https://novel18.syosetu.com/n3161kd/1/
	// 正規表現でエピソード番号を抽出
	re := regexp.MustCompile(`/n[0-9]+[a-z]+/([0-9]+)/?`)
	matches := re.FindStringSubmatch(url)
	if len(matches) >= 2 {
		return matches[1]
	}

	// フォールバック：従来の方法
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "n") && len(part) > 1 {
			if i+1 < len(parts) && parts[i+1] != "" && parts[i+1] != "/" {
				// 数字のみかチェック
				episodeNum := parts[i+1]
				isNumeric := true
				for _, char := range episodeNum {
					if char < '0' || char > '9' {
						isNumeric = false
						break
					}
				}
				if isNumeric {
					return episodeNum
				}
			}
		}
	}
	return "1" // デフォルトはエピソード1
}

// generateFileName は小説番号とエピソード番号からファイル名を生成します
func generateFileName(novelCode, episodeNumber string) string {
	return fmt.Sprintf("%s-%s", novelCode, episodeNumber)
}

// isFileAlreadySaved は指定されたファイルが既に保存されているかチェックします
func (a *App) isFileAlreadySaved(savePath, fileName, episodeNumber string, createHtml, createTxt bool) (bool, bool) {
	var htmlExists, txtExists bool

	// HTMLファイルの存在チェック（エピソード番号のみをファイル名に使用）
	if createHtml {
		htmlPath := filepath.Join(savePath, "html", episodeNumber+".html")
		if _, err := os.Stat(htmlPath); err == nil {
			htmlExists = true
		}
	} else {
		htmlExists = true // HTMLを作成しない場合は常にtrue
	}

	// TXTファイルの存在チェック（従来通り小説番号-エピソード番号）
	if createTxt {
		txtPath := filepath.Join(savePath, fileName+".txt")
		if _, err := os.Stat(txtPath); err == nil {
			txtExists = true
		}
	} else {
		txtExists = true // TXTを作成しない場合は常にtrue
	}

	return htmlExists, txtExists
}

// shouldSkipEpisode はエピソードをスキップするかどうかを判定します
func (a *App) shouldSkipEpisode(savePath, fileName, episodeNumber string, createHtml, createTxt bool) bool {
	htmlExists, txtExists := a.isFileAlreadySaved(savePath, fileName, episodeNumber, createHtml, createTxt)

	// 必要なファイルがすべて存在する場合はスキップ
	return htmlExists && txtExists
}

// GetTitle は小説のタイトルを取得します（フロントエンド用）
func (a *App) GetTitle(url string) (string, error) {
	// 各話URLの場合は小説インデックスURLに変換
	processedURL := a.convertToIndexURL(url)

	result := a.StartScraping(processedURL)
	if result.Error != "" {
		return "", fmt.Errorf(result.Error)
	}
	return result.Title, nil
}

// generateEpisodeHTML はエピソード用HTMLを生成します
func (a *App) generateEpisodeHTML(episodeTitle, content, novelTitle string, episodeNum, totalEpisodes int) string {
	// 改行をHTMLの<br>タグに変換
	htmlContent := strings.ReplaceAll(content, "\n", "<br>")

	// ナビゲーションリンクの生成
	var prevLink, nextLink string
	if episodeNum > 1 {
		prevLink = fmt.Sprintf(`<a href="%d.html">← 前のエピソード</a>`, episodeNum-1)
	}
	if episodeNum < totalEpisodes {
		nextLink = fmt.Sprintf(`<a href="%d.html">次のエピソード →</a>`, episodeNum+1)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - %s</title>
    <style>
        body { font-family: 'Hiragino Kaku Gothic Pro', 'ヒラギノ角ゴ Pro W3', Meiryo, メイリオ, Osaka, 'MS PGothic', arial, helvetica, sans-serif; line-height: 1.6; margin: 40px; max-width: 800px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; border-bottom: 2px solid #333; padding-bottom: 10px; }
        .nav { margin: 20px 0; text-align: center; }
        .nav a { display: inline-block; margin: 0 10px; padding: 8px 16px; background: #f0f0f0; text-decoration: none; color: #333; border-radius: 4px; }
        .nav a:hover { background: #e0e0e0; }
        .content { margin: 20px 0; }
        .back-to-index { text-align: center; margin: 30px 0; }
        .back-to-index a { padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; }
        .back-to-index a:hover { background: #0056b3; }
    </style>
</head>
<body>
    <h1>%s</h1>
    
    <div class="nav">
        %s
        %s
    </div>
    
    <div class="content">
        %s
    </div>
    
    <div class="nav">
        %s
        %s
    </div>
    
    <div class="back-to-index">
        <a href="../index-1.html">← エピソード一覧に戻る</a>
    </div>
</body>
</html>`, episodeTitle, novelTitle, episodeTitle, prevLink, nextLink, htmlContent, prevLink, nextLink)

	return html
}

// formatChapterContent は各話のテキストコンテンツをフォーマットします
func (a *App) formatChapterContent(novelTitle, author, chapterTitle, content string) string {
	var formatted strings.Builder

	// ルビを青空文庫形式に変換
	content = a.convertRubyToAozora(content)

	// 各話のタイトル（短編の場合でもタイトルを表示）
	if chapterTitle != "" {
		// タイトルのルビも変換
		convertedTitle := a.convertRubyToAozora(chapterTitle)
		formatted.WriteString(convertedTitle)
		formatted.WriteString("\n\n")
	} else {
		// 短編の場合は小説タイトルを使用
		convertedNovelTitle := a.convertRubyToAozora(novelTitle)
		formatted.WriteString(convertedNovelTitle)
		formatted.WriteString("\n\n")
	}

	// 本文（各行にインデントを追加）
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			formatted.WriteString(line)
		}
		formatted.WriteString("\n")
	}

	return formatted.String()
}

// convertRubyToAozora はHTMLのrubyタグを青空文庫形式に変換します
func (a *App) convertRubyToAozora(content string) string {
	rubyPattern := regexp.MustCompile(`<ruby>(.*?)<rp>.*?</rp><rt>(.*?)</rt><rp>.*?</rp></ruby>`)
	content = rubyPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatch := rubyPattern.FindStringSubmatch(match)
		if len(submatch) >= 3 {
			baseText := submatch[1]
			ruby := submatch[2]
			return a.formatAozoraRuby(baseText, ruby, match)
		}
		return match
	})

	simpleRubyPattern := regexp.MustCompile(`<ruby>(.*?)<rt>(.*?)</rt></ruby>`)
	content = simpleRubyPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatch := simpleRubyPattern.FindStringSubmatch(match)
		if len(submatch) >= 3 {
			baseText := submatch[1]
			ruby := submatch[2]
			return a.formatAozoraRuby(baseText, ruby, match)
		}
		return match
	})

	return content
}

// formatAozoraRuby は青空文庫形式のルビを正しくフォーマットします
func (a *App) formatAozoraRuby(baseText, ruby, originalMatch string) string {
	// ルビのかかる文字列が漢字のみか確認
	isKanjiOnly := regexp.MustCompile(`^[\p{Han}々仝〆〇ヶ]+$`).MatchString(baseText)

	if isKanjiOnly {
		// 漢字のみの場合、｜は不要
		return baseText + "《" + ruby + "》"
	} else {
		// 漢字以外が含まれる場合、｜を付ける
		return "｜" + baseText + "《" + ruby + "》"
	}
}

// formatChapterContentForCombined は連結ファイル用に各話のテキストコンテンツをフォーマットします（タイトル・作者名なし）
func (a *App) formatChapterContentForCombined(chapterTitle, content string) string {
	var formatted strings.Builder

	// ルビを青空文庫形式に変換
	content = a.convertRubyToAozora(content)

	// 各話のタイトル（ルビ変換済み）
	convertedTitle := a.convertRubyToAozora(chapterTitle)
	formatted.WriteString(convertedTitle)
	formatted.WriteString("\n\n")

	// 本文（各行にインデントを追加）
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			formatted.WriteString(line)
		}
		formatted.WriteString("\n")
	}

	return formatted.String()
}

// convertToIndexURL は各話URLを小説インデックスURLに変換します
func (a *App) convertToIndexURL(url string) string {
	// ncode.syosetu.com用の正規表現
	ncodePattern := regexp.MustCompile(`^(https://ncode\.syosetu\.com/n[0-9]+[a-z]+)/([0-9]+)/?$`)
	matches := ncodePattern.FindStringSubmatch(url)

	if len(matches) >= 2 {
		// 各話URLの場合、インデックスURLに変換
		indexURL := matches[1] + "/"
		return indexURL
	}

	// novel18.syosetu.com用の正規表現
	novel18Pattern := regexp.MustCompile(`^(https://novel18\.syosetu\.com/n[0-9]+[a-z]+)/([0-9]+)/?$`)
	matches = novel18Pattern.FindStringSubmatch(url)

	if len(matches) >= 2 {
		// 各話URLの場合、インデックスURLに変換
		indexURL := matches[1] + "/"
		return indexURL
	}

	// 各話URLでない場合はそのまま返す
	return url
}

// generateEpisodeHTMLWithOriginalStructure は元のHTML構造を保った上でエピソード用HTMLを生成します
func (a *App) generateEpisodeHTMLWithOriginalStructure(episodeTitle, rawHTML, novelTitle string, episodeNum, totalEpisodes int) string {
	// ナビゲーションリンクの生成
	var prevLink, nextLink string
	if episodeNum > 1 {
		prevLink = fmt.Sprintf(`<a href="%d.html">← 前のエピソード</a>`, episodeNum-1)
	}
	if episodeNum < totalEpisodes {
		nextLink = fmt.Sprintf(`<a href="%d.html">次のエピソード →</a>`, episodeNum+1)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - %s</title>
    <style>
        /* 元サイトのスタイルを模擬 */
        body { 
            font-family: 'Hiragino Kaku Gothic Pro', 'ヒラギノ角ゴ Pro W3', Meiryo, メイリオ, Osaka, 'MS PGothic', arial, helvetica, sans-serif; 
            line-height: 1.7; 
            color: #333; 
            background-color: #fff;
            max-width: 800px; 
            margin: 0 auto; 
            padding: 20px; 
        }
        
        /* ナビゲーション */
        .nav { 
            margin: 20px 0; 
            text-align: center;
            padding: 15px 0;
            border-top: 1px solid #ddd;
            border-bottom: 1px solid #ddd;
        }
        .nav a { 
            display: inline-block; 
            margin: 0 15px; 
            padding: 10px 20px; 
            background: #f8f9fa; 
            text-decoration: none; 
            color: #495057; 
            border-radius: 6px;
            border: 1px solid #dee2e6;
            transition: all 0.2s;
        }
        .nav a:hover { 
            background: #e9ecef; 
            color: #212529;
        }
        
        /* 小説本文エリア */
        .p-novel__body {
            margin: 30px 0;
            line-height: 1.8;
        }
        
        /* 本文テキスト */
        .p-novel__text {
            margin: 1.5em 0;
            text-align: left;
        }
        
        /* 改ページ */
        .js-novel-text-br {
            height: 1em;
        }
        
        /* ルビ */
        ruby {
            ruby-align: center;
        }
        
        rt {
            font-size: 0.7em;
        }
        
        /* 傍点 */
        .emphasis {
            text-emphasis: filled circle;
            -webkit-text-emphasis: filled circle;
        }
        
        h1 { 
            color: #333; 
            border-bottom: 2px solid #007bff; 
            padding-bottom: 10px;
            margin-bottom: 30px;
        }
        
        .back-to-index { 
            text-align: center; 
            margin: 40px 0; 
        }
        .back-to-index a { 
            padding: 12px 24px; 
            background: #007bff; 
            color: white; 
            text-decoration: none; 
            border-radius: 6px;
            transition: background-color 0.2s;
        }
        .back-to-index a:hover { 
            background: #0056b3; 
        }
    </style>
</head>
<body>
    <h1>%s</h1>
    
    <div class="nav">
        %s
        %s
    </div>
    
    <div class="p-novel__body">
        %s
    </div>
    
    <div class="nav">
        %s
        %s
    </div>
    
    <div class="back-to-index">
        <a href="../index-1.html">← エピソード一覧に戻る</a>
    </div>
</body>
</html>`, episodeTitle, novelTitle, episodeTitle, prevLink, nextLink, rawHTML, prevLink, nextLink)

	return html
}

// createIndexPages はインデックスページを作成します（ページング対応）
func (a *App) createIndexPages(savePath, novelTitle string, chapters []ChapterInfo) error {
	const episodesPerPage = 50 // 1ページあたりのエピソード数
	totalPages := (len(chapters) + episodesPerPage - 1) / episodesPerPage

	for page := 1; page <= totalPages; page++ {
		startIdx := (page - 1) * episodesPerPage
		endIdx := startIdx + episodesPerPage
		if endIdx > len(chapters) {
			endIdx = len(chapters)
		}

		pageChapters := chapters[startIdx:endIdx]

		// インデックスページのHTML生成
		indexHTML := a.generateIndexHTML(novelTitle, pageChapters, page, totalPages, startIdx)

		// ファイル保存
		fileName := fmt.Sprintf("index-%d.html", page)
		filePath := filepath.Join(savePath, fileName)
		if err := os.WriteFile(filePath, []byte(indexHTML), 0644); err != nil {
			return fmt.Errorf("インデックスページ%dの保存に失敗しました: %w", page, err)
		}
	}

	return nil
}

// generateIndexHTML はインデックスページ用HTMLを生成します
func (a *App) generateIndexHTML(novelTitle string, chapters []ChapterInfo, currentPage, totalPages, startIdx int) string {
	var episodeList strings.Builder

	for i, chapter := range chapters {
		episodeNum := startIdx + i + 1
		// URLからエピソード番号を抽出（HTMLファイル名として使用）
		episodeNumber := extractEpisodeNumberFromURL(chapter.URL)
		if episodeNumber == "" || (i > 0 && episodeNumber == "1") {
			episodeNumber = fmt.Sprintf("%d", episodeNum)
		}
		episodeList.WriteString(fmt.Sprintf(`        <li><a href="html/%s.html">第%d話 %s</a></li>
`, episodeNumber, episodeNum, chapter.Title))
	}

	// ページネーション
	var pagination strings.Builder
	if currentPage > 1 {
		pagination.WriteString(fmt.Sprintf(`        <a href="index-%d.html">← 前のページ</a>
`, currentPage-1))
	}
	if currentPage < totalPages {
		pagination.WriteString(fmt.Sprintf(`        <a href="index-%d.html">次のページ →</a>
`, currentPage+1))
	}

	// ページリンク
	var pageLinks strings.Builder
	for i := 1; i <= totalPages; i++ {
		if i == currentPage {
			pageLinks.WriteString(fmt.Sprintf(`        <span class="current-page">%d</span>
`, i))
		} else {
			pageLinks.WriteString(fmt.Sprintf(`        <a href="index-%d.html">%d</a>
`, i, i))
		}
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - エピソード一覧 (ページ%d)</title>
    <style>
        body { font-family: 'Hiragino Kaku Gothic Pro', 'ヒラギノ角ゴ Pro W3', Meiryo, メイリオ, Osaka, 'MS PGothic', arial, helvetica, sans-serif; line-height: 1.6; margin: 40px; max-width: 800px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; border-bottom: 2px solid #333; padding-bottom: 10px; }
        .page-info { text-align: center; margin: 20px 0; color: #666; }
        ul { list-style-type: none; padding: 0; }
        li { margin: 8px 0; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        li:hover { background-color: #f9f9f9; }
        a { text-decoration: none; color: #007bff; }
        a:hover { text-decoration: underline; }
        .pagination { text-align: center; margin: 30px 0; }
        .pagination a, .pagination span { display: inline-block; margin: 0 5px; padding: 8px 12px; border: 1px solid #ddd; text-decoration: none; color: #007bff; border-radius: 4px; }
        .pagination a:hover { background-color: #e9ecef; }
        .pagination .current-page { background-color: #007bff; color: white; border-color: #007bff; }
        .page-links { text-align: center; margin: 20px 0; }
        .page-links a, .page-links span { margin: 0 2px; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <div class="page-info">ページ %d / %d</div>
    
    <ul>
%s    </ul>
    
    <div class="pagination">
%s    </div>
    
    <div class="page-links">
%s    </div>
</body>
</html>`, novelTitle, currentPage, novelTitle, currentPage, totalPages, episodeList.String(), pagination.String(), pageLinks.String())

	return html
}

// saveOriginalIndexPages は元のHTMLを使用してインデックスページを保存します
func (a *App) saveOriginalIndexPages(savePath string, indexPagesHTML []string, chapters []ChapterInfo) error {
	for i, pageHTML := range indexPagesHTML {
		// iframeタグを除去
		cleanHTML := a.removeIframes(pageHTML)

		// エピソードリンクをローカルファイルリンクに変換
		modifiedHTML := a.convertEpisodeLinksToLocal(cleanHTML, chapters)

		fileName := fmt.Sprintf("index-%d.html", i+1)
		filePath := filepath.Join(savePath, fileName)
		if err := os.WriteFile(filePath, []byte(modifiedHTML), 0644); err != nil {
			return fmt.Errorf("インデックスページ%dの保存に失敗しました: %w", i+1, err)
		}
	}
	return nil
}

// convertEpisodeLinksToLocal はエピソードリンクをローカルファイルリンクに変換します
func (a *App) convertEpisodeLinksToLocal(html string, chapters []ChapterInfo) string {
	// エピソードURLとローカルファイル名のマッピングを作成
	for _, chapter := range chapters {
		// 元のエピソードURL
		originalURL := chapter.URL

		// ローカルファイル名を生成
		episodeNumber := extractEpisodeNumberFromURL(originalURL)
		if episodeNumber == "" {
			continue
		}

		// 相対パス形式のローカルリンク
		localLink := fmt.Sprintf("html/%s.html", episodeNumber)

		// 正規表現でより柔軟なリンク置換
		// 完全URL形式のパターン
		fullURLPattern := regexp.MustCompile(regexp.QuoteMeta(originalURL))
		html = fullURLPattern.ReplaceAllString(html, localLink)

		// 相対パス形式のパターン（例：/n2532kp/1/）
		if strings.Contains(originalURL, "ncode.syosetu.com") {
			// URLから相対パス部分を抽出
			re := regexp.MustCompile(`https://ncode\.syosetu\.com(/n[0-9]+[a-z]+/[0-9]+/?)`)
			matches := re.FindStringSubmatch(originalURL)
			if len(matches) > 1 {
				relativePath := matches[1]
				// 相対パスのリンクも置換
				relativePattern := regexp.MustCompile(fmt.Sprintf(`href="(%s)"`, regexp.QuoteMeta(relativePath)))
				html = relativePattern.ReplaceAllString(html, fmt.Sprintf(`href="%s"`, localLink))
			}
		}
	}

	return html
}

// removeIframes はHTMLからすべてのiframeタグを除去します
func (a *App) removeIframes(html string) string {
	// iframeタグ（開始タグから終了タグまで）を正規表現で除去
	iframePattern := regexp.MustCompile(`(?i)<iframe[^>]*>.*?</iframe>`)
	html = iframePattern.ReplaceAllString(html, "")

	// 自己完結型のiframeタグも除去
	selfClosingIframePattern := regexp.MustCompile(`(?i)<iframe[^>]*/>`)
	html = selfClosingIframePattern.ReplaceAllString(html, "")

	return html
}

// generateShortNovelHTML は短編小説用のHTMLを生成します
func (a *App) generateShortNovelHTML(title, rawHTML string) string {
	html := fmt.Sprintf(`<\!DOCTYPE html>
<html lang="ja">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        /* 元サイトのスタイルを模擬 */
        body { 
            font-family: "Hiragino Kaku Gothic Pro", "ヒラギノ角ゴ Pro W3", Meiryo, メイリオ, Osaka, "MS PGothic", arial, helvetica, sans-serif; 
            line-height: 1.7; 
            color: #333; 
            background-color: #fff;
            max-width: 800px; 
            margin: 0 auto; 
            padding: 20px; 
        }
        
        /* 小説本文エリア */
        .p-novel__body {
            margin: 30px 0;
            line-height: 1.8;
        }
        
        /* 本文テキスト */
        .p-novel__text {
            margin: 1.5em 0;
            text-align: left;
        }
        
        /* 改ページ */
        .js-novel-text-br {
            height: 1em;
        }
        
        /* ルビ */
        ruby {
            ruby-align: center;
        }
        
        rt {
            font-size: 0.7em;
        }
        
        /* 傍点 */
        .emphasis {
            text-emphasis: filled circle;
            -webkit-text-emphasis: filled circle;
        }
        
        h1 { 
            color: #333; 
            border-bottom: 2px solid #007bff; 
            padding-bottom: 10px;
            margin-bottom: 30px;
        }
    </style>
</head>
<body>
    <h1>%s</h1>
    
    <div class="p-novel__body">
        %s
    </div>
</body>
</html>`, title, title, rawHTML)

	return html
}
