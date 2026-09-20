package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func htmlOnlyOptions() map[string]interface{} {
	return map[string]interface{}{
		"createHtml":     true,
		"createTxt":      false,
		"createReadable": false,
	}
}

// 連載は1話とし、実サイトへ接続せず通常のダウンロード経路を検証する。
func newHTMLNovelServer(t *testing.T, serial bool, body string) (string, *atomic.Int32) {
	t.Helper()
	chapterRequests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/n123ab/":
			if serial {
				fmt.Fprintf(w, `<html><body><h1>作品</h1><div class="p-novel__author">作者</div><div class="p-eplist"><div class="p-eplist__sublist"><a href="http://%s/n123ab/1/">第一話</a></div></div></body></html>`, r.Host)
				return
			}
		case "/n123ab/1/":
			chapterRequests.Add(1)
		default:
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `<!DOCTYPE html><html><head><link rel="stylesheet" href="/novel.css"></head><body><h1>作品</h1><div class="p-novel__author">作者</div>`+body+`<footer>保存対象外</footer></body></html>`)
	}))
	t.Cleanup(server.Close)
	return server.URL + "/n123ab/", chapterRequests
}

func assertHTMLBytes(t *testing.T, savePath, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(savePath, "N123AB-1.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("保存したHTMLが原文と異なります\ngot:  %q\nwant: %q", got, want)
	}
}

func TestHTMLOutputPreservesMainSource(t *testing.T) {
	mainHTML := "<MAIN data-z='2' class=story data-a=\"1\">\r\n" +
		"<!-- 原文の空白・引用符・文字参照を保持 -->\n" +
		"<div class='p-novel__body'>漢字 &amp; &#x3042; &nbsp;<ruby>空<rt>そら</rt></ruby></div>\r" +
		"<a href='/n123ab/2/'>次へ</a><img src='/illustration.jpg'><iframe src='/frame'></iframe>\r\n" +
		"<script>const example = '</main>';</script></MAIN>"
	for _, serial := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			t.Run(fmt.Sprintf("serial=%t/encoding設定=%t", serial, configured), func(t *testing.T) {
				url, _ := newHTMLNovelServer(t, serial, "\n"+mainHTML+"\r\n")
				options := htmlOnlyOptions()
				if configured {
					options["encoding"] = "UTF-16LE"
					options["lineEnding"] = "CR"
				}
				options["createCombined"] = true
				savePath := t.TempDir()
				if err := NewApp().DownloadNovel(url, savePath, options); err != nil {
					t.Fatal(err)
				}
				assertHTMLBytes(t, savePath, mainHTML)
				entries, err := os.ReadDir(savePath)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 || entries[0].Name() != "N123AB-1.html" {
					t.Fatalf("HTML単独保存で想定外の出力があります: %v", entries)
				}
			})
		}
	}
}

func TestHTMLOutputWithBothTextFormats(t *testing.T) {
	mainHTML := `<main><div class="p-novel__body"><div class="p-novel__text">これは<ruby>漢字<rt>かんじ</rt></ruby>です。</div></div></main>`
	for _, serial := range []bool{false, true} {
		t.Run(fmt.Sprintf("serial=%t", serial), func(t *testing.T) {
			url, _ := newHTMLNovelServer(t, serial, mainHTML)
			savePath := t.TempDir()
			options := htmlOnlyOptions()
			options["createTxt"] = true
			options["createReadable"] = true
			if err := NewApp().DownloadNovel(url, savePath, options); err != nil {
				t.Fatal(err)
			}
			assertHTMLBytes(t, savePath, mainHTML)
			for name, want := range map[string]string{
				"N123AB-1.txt":        "漢字（かんじ）",
				"aozora-N123AB-1.txt": "漢字《かんじ》",
			} {
				data, err := os.ReadFile(filepath.Join(savePath, name))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), want) {
					t.Fatalf("%s の内容に %q がありません: %q", name, want, data)
				}
			}
		})
	}
}

func TestHTMLOutputTextOnlyAllowsMissingMain(t *testing.T) {
	body := `<div class="p-novel__body"><div class="p-novel__text">本文です。</div></div>`
	for _, serial := range []bool{false, true} {
		t.Run(fmt.Sprintf("serial=%t", serial), func(t *testing.T) {
			url, _ := newHTMLNovelServer(t, serial, body)
			savePath := t.TempDir()
			options := htmlOnlyOptions()
			options["createHtml"] = false
			options["createReadable"] = true
			if err := NewApp().DownloadNovel(url, savePath, options); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(savePath, "N123AB-1.txt"))
			if err != nil || !strings.Contains(string(data), "本文です。") {
				t.Fatalf("mainがないページのTXT保存に失敗しました: data=%q err=%v", data, err)
			}
			if _, err := os.Stat(filepath.Join(savePath, "N123AB-1.html")); !os.IsNotExist(err) {
				t.Fatalf("未選択のHTMLが作成されました: %v", err)
			}
		})
	}
}

func TestHTMLOutputMissingMainFailsWithoutFile(t *testing.T) {
	for _, serial := range []bool{false, true} {
		t.Run(fmt.Sprintf("serial=%t", serial), func(t *testing.T) {
			url, requests := newHTMLNovelServer(t, serial, `<div class="p-novel__body">mainなし</div>`)
			savePath := t.TempDir()
			if err := NewApp().DownloadNovel(url, savePath, htmlOnlyOptions()); err == nil {
				t.Fatal("main要素がないHTMLの保存が成功扱いになりました")
			}
			entries, err := os.ReadDir(savePath)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("main抽出失敗後にファイルが残りました: %v", entries)
			}
			if serial && requests.Load() != 3 {
				t.Fatalf("連載各話の取得試行回数 = %d, want 3", requests.Load())
			}
		})
	}
}

func TestHTMLOutputSavedFileSkipsBothNovelTypes(t *testing.T) {
	for _, serial := range []bool{false, true} {
		t.Run(fmt.Sprintf("serial=%t", serial), func(t *testing.T) {
			url, requests := newHTMLNovelServer(t, serial, `<main><div class="p-novel__body">新しい本文</div></main>`)
			savePath := t.TempDir()
			const saved = "<main>保存済みの本文</main>"
			if err := os.WriteFile(filepath.Join(savePath, "N123AB-1.html"), []byte(saved), 0644); err != nil {
				t.Fatal(err)
			}
			if err := NewApp().DownloadNovel(url, savePath, htmlOnlyOptions()); err != nil {
				t.Fatal(err)
			}
			assertHTMLBytes(t, savePath, saved)
			if serial && requests.Load() != 0 {
				t.Fatalf("保存済みの連載各話を再取得しました: %d回", requests.Load())
			}
		})
	}
}

func TestHTMLOutputSavedCheckRequiresNonemptyRegularFile(t *testing.T) {
	for _, kind := range []string{"empty", "directory", "legacyPath"} {
		t.Run(kind, func(t *testing.T) {
			savePath := t.TempDir()
			filePath := filepath.Join(savePath, "N123AB-1.html")
			var err error
			switch kind {
			case "empty":
				err = os.WriteFile(filePath, nil, 0644)
			case "directory":
				err = os.Mkdir(filePath, 0755)
			case "legacyPath":
				err = os.Mkdir(filepath.Join(savePath, "html"), 0755)
				if err == nil {
					err = os.WriteFile(filepath.Join(savePath, "html", "1.html"), []byte("<main>旧ファイル</main>"), 0644)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if NewApp().shouldSkipEpisode(savePath, "N123AB-1", "1", true, false, false) {
				t.Fatal("新形式のHTMLがないのに保存済みと判定されました")
			}
		})
	}
	// HTMLがあっても、併用するTXTが未保存ならエピソード全体をスキップしない。
	savePath := t.TempDir()
	if err := os.WriteFile(filepath.Join(savePath, "N123AB-1.html"), []byte("<main>本文</main>"), 0644); err != nil {
		t.Fatal(err)
	}
	if NewApp().shouldSkipEpisode(savePath, "N123AB-1", "1", true, false, true) {
		t.Fatal("未保存の読書用TXTがあるのにスキップされました")
	}
}

func TestHTMLOutputSerialSaveFailureReturnsError(t *testing.T) {
	url, _ := newHTMLNovelServer(t, true, `<main><div class="p-novel__body">本文</div></main>`)
	savePath := t.TempDir()
	filePath := filepath.Join(savePath, "N123AB-1.html")
	if err := os.Mkdir(filePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := NewApp().DownloadNovel(url, savePath, htmlOnlyOptions()); err == nil {
		t.Fatal("連載各話のHTML保存失敗が成功扱いになりました")
	}
	info, err := os.Stat(filePath)
	if err != nil || !info.IsDir() {
		t.Fatalf("保存先にあったディレクトリが変更されました: %v", err)
	}
	tempFiles, err := filepath.Glob(filepath.Join(savePath, ".narou-html-*"))
	if err != nil || len(tempFiles) != 0 {
		t.Fatalf("保存失敗後に一時ファイルが残りました: files=%v err=%v", tempFiles, err)
	}
}

func TestHTMLOutputResumePreservesCombinedText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/n123ab/":
			fmt.Fprintf(w, `<h1>作品</h1><div class="p-eplist"><div class="p-eplist__sublist"><a href="http://%s/n123ab/1/">第一話</a></div><div class="p-eplist__sublist"><a href="http://%s/n123ab/2/">第二話</a></div></div>`, r.Host, r.Host)
		case "/n123ab/2/":
			fmt.Fprint(w, `<main><div class="p-novel__body"><div class="p-novel__text">第二話の本文</div></div></main>`)
		default:
			t.Errorf("保存済みの話への想定外のリクエスト: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	savePath := t.TempDir()
	for name, content := range map[string]string{
		"N123AB-1.html": "<main>第一話</main>",
		"N123AB-1.txt":  "第一話の本文",
		"all.txt":       "第一話の本文\r\n第二話の本文\r\n",
	} {
		if err := os.WriteFile(filepath.Join(savePath, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	options := htmlOnlyOptions()
	options["createReadable"] = true
	options["createCombined"] = true
	if err := NewApp().DownloadNovel(server.URL+"/n123ab/", savePath, options); err != nil {
		t.Fatal(err)
	}
	combined, err := os.ReadFile(filepath.Join(savePath, "all.txt"))
	if err != nil || string(combined) != "第一話の本文\r\n第二話の本文\r\n" {
		t.Fatalf("再取得した話だけで連結TXTが変更されました: %q, %v", combined, err)
	}
	if _, err := os.Stat(filepath.Join(savePath, "N123AB-2.html")); err != nil {
		t.Fatalf("未保存のHTMLが取得されていません: %v", err)
	}
}
