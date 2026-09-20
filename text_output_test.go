package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestTextOutput_SaveBothFormats(t *testing.T) {
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "Shift-JIS"} {
		for _, baseName := range []string{"N123AB-1", "all"} {
			t.Run(encoding+"/"+baseName, func(t *testing.T) {
				app := NewApp()
				savePath := t.TempDir()
				aozora := "第一話\n\n漢字《かんじ》と［＃傍点］重要［＃傍点終わり］\n"
				readable := "第一話\n\n漢字（かんじ）と重要\n"
				expectedPath := t.TempDir()
				wantAozora := writeTextOutputFixture(t, app, expectedPath, "aozora", aozora, encoding, "CR+LF")
				wantReadable := writeTextOutputFixture(t, app, expectedPath, "readable", readable, encoding, "CR+LF")

				if err := app.saveTextFileWithRetry(savePath, baseName, aozora, encoding, "CR+LF", textFormatAozora); err != nil {
					t.Fatalf("青空文庫TXTの保存に失敗: %v", err)
				}
				if err := app.saveTextFileWithRetry(savePath, baseName, readable, encoding, "CR+LF", textFormatReadable); err != nil {
					t.Fatalf("読書用TXTの保存に失敗: %v", err)
				}

				assertTextOutputFile(t, filepath.Join(savePath, "aozora-"+baseName+".txt"), wantAozora)
				assertTextOutputFile(t, filepath.Join(savePath, baseName+".txt"), wantReadable)
				entries, err := os.ReadDir(savePath)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 2 {
					t.Fatalf("保存ファイル数 = %d, want 2: %v", len(entries), entries)
				}
				if !app.shouldSkipEpisode(savePath, baseName, "1", false, true, true) {
					t.Fatal("両形式が新しい名前で保存済みならスキップする必要があります")
				}
			})
		}
	}
}

func TestTextOutput_SkipDistinguishesLegacyAozora(t *testing.T) {
	legacyContents := []struct {
		name string
		text string
	}{
		{"漢字ルビ", "第一話\n\n漢字《かんじ》です。\n"},
		{"親文字マーカー付きルビ", "第一話\n\n｜ABC《えーびーしー》です。\n"},
		{"青空文庫注記", "第一話\n\n［＃傍点］重要［＃傍点終わり］です。\n"},
	}
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "Shift-JIS"} {
		for _, legacy := range legacyContents {
			t.Run(encoding+"/"+legacy.name, func(t *testing.T) {
				app := NewApp()
				savePath := t.TempDir()
				baseName := "N123AB-1"
				readable := "第一話\n\n漢字（かんじ）とABC（えーびーしー）と重要です。\n"
				writeTextOutputFixture(t, app, savePath, baseName, legacy.text, encoding, "CR+LF")
				writeTextOutputFixture(t, app, savePath, baseName+"-readable", readable, encoding, "CR+LF")

				_, aozoraExists, readableExists := app.isFileAlreadySaved(savePath, baseName, "1", false, true, true)
				if aozoraExists || readableExists {
					t.Fatalf("旧名だけの保存済み判定 = (aozora:%v, readable:%v), want (false, false)", aozoraExists, readableExists)
				}
				if app.shouldSkipEpisode(savePath, baseName, "1", false, false, true) {
					t.Fatal("旧青空文庫TXTを読書用TXTとしてスキップしてはいけません")
				}

				writeTextOutputFixture(t, app, savePath, baseName, readable, encoding, "CR+LF")
				if !app.shouldSkipEpisode(savePath, baseName, "1", false, false, true) {
					t.Fatal("新しい通常名の読書用TXTはスキップする必要があります")
				}
				if app.shouldSkipEpisode(savePath, baseName, "1", false, true, true) {
					t.Fatal("青空文庫TXTがないため両形式の取得はスキップできません")
				}

				writeTextOutputFixture(t, app, savePath, "aozora-"+baseName, legacy.text, encoding, "CR+LF")
				if !app.shouldSkipEpisode(savePath, baseName, "1", false, true, true) {
					t.Fatal("新しい両形式のファイルが揃えばスキップする必要があります")
				}
			})
		}
	}
}

func TestTextOutput_ReadablePreservesLegacyAozora(t *testing.T) {
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "Shift-JIS"} {
		for _, baseName := range []string{"N123AB-1", "all"} {
			t.Run(encoding+"/"+baseName, func(t *testing.T) {
				app := NewApp()
				savePath := t.TempDir()
				legacy := writeTextOutputFixture(t, app, savePath, baseName, "旧本文\n\n漢字《かんじ》です。\n", encoding, "CR+LF")
				legacyReadable := writeTextOutputFixture(t, app, savePath, baseName+"-readable", "旧読書用本文\n", encoding, "CR+LF")
				readable := "更新した本文\n\n漢字（かんじ）です。\n"
				wantReadable := writeTextOutputFixture(t, app, t.TempDir(), "expected", readable, "UTF-8", "LF")

				// 新保存の文字コード・改行を変更しても、退避する旧ファイルのバイト列は維持する。
				if err := app.saveTextFileWithRetry(savePath, baseName, readable, "UTF-8", "LF", textFormatReadable); err != nil {
					t.Fatalf("読書用TXTの保存に失敗: %v", err)
				}

				assertTextOutputFile(t, filepath.Join(savePath, baseName+".txt"), wantReadable)
				assertTextOutputFile(t, filepath.Join(savePath, "aozora-"+baseName+".txt"), legacy)
				assertTextOutputFile(t, filepath.Join(savePath, baseName+"-readable.txt"), legacyReadable)
				if !app.shouldSkipEpisode(savePath, baseName, "1", false, true, true) {
					t.Fatal("退避した青空文庫TXTと新しい読書用TXTは両方とも保存済みです")
				}
			})
		}
	}
}

func TestTextOutput_BackupPreservesExistingPrefixedFiles(t *testing.T) {
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "Shift-JIS"} {
		for _, baseName := range []string{"N123AB-1", "all"} {
			t.Run(encoding+"/"+baseName, func(t *testing.T) {
				app := NewApp()
				savePath := t.TempDir()
				legacy := writeTextOutputFixture(t, app, savePath, baseName, "退避対象\n漢字《かんじ》\n", encoding, "CR+LF")
				prefixed := writeTextOutputFixture(t, app, savePath, "aozora-"+baseName, "既存の青空文庫\n本文《ほんぶん》\n", encoding, "CR+LF")
				previous := writeTextOutputFixture(t, app, savePath, "aozora-"+baseName+"-previous", "さらに古い青空文庫\n本文《ほんぶん》\n", encoding, "CR+LF")
				readable := "新しい本文\n漢字（かんじ）\n"
				wantReadable := writeTextOutputFixture(t, app, t.TempDir(), "expected", readable, encoding, "CR+LF")

				if err := app.saveTextFileWithRetry(savePath, baseName, readable, encoding, "CR+LF", textFormatReadable); err != nil {
					t.Fatalf("退避先が存在する場合の保存に失敗: %v", err)
				}
				assertTextOutputFile(t, filepath.Join(savePath, baseName+".txt"), wantReadable)
				assertTextOutputFile(t, filepath.Join(savePath, "aozora-"+baseName+".txt"), prefixed)
				assertTextOutputFile(t, filepath.Join(savePath, "aozora-"+baseName+"-previous.txt"), previous)
				assertTextOutputFile(t, filepath.Join(savePath, "aozora-"+baseName+"-previous-2.txt"), legacy)

				if err := app.saveTextFileWithRetry(savePath, baseName, readable, encoding, "CR+LF", textFormatReadable); err != nil {
					t.Fatalf("読書用TXTの再保存に失敗: %v", err)
				}
				entries, err := os.ReadDir(savePath)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 4 {
					t.Fatalf("読書用TXTの再保存で余分な退避が発生: %v", entries)
				}
			})
		}
	}
}

func writeTextOutputFixture(t *testing.T, app *App, savePath, title, content, encoding, lineEnding string) []byte {
	t.Helper()
	if err := app.saveTextFile(savePath, title, content, encoding, lineEnding); err != nil {
		t.Fatalf("テスト用TXTの作成に失敗: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(savePath, title+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertTextOutputFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("保存ファイル %s の読み込みに失敗: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("保存ファイル %s の内容が不一致:\ngot:  %x\nwant: %x", path, got, want)
	}
}
