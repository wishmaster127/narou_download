package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// saveMainHTMLWithRetry はmain原文を変換せず、書き込み完了後に保存先へ移します。
func (a *App) saveMainHTMLWithRetry(savePath, fileName, mainHTML string) error {
	if mainHTML == "" {
		return fmt.Errorf("保存するmain要素がありません")
	}
	filePath := filepath.Join(savePath, sanitizeFileName(fileName)+".html")
	const maxRetries = 3
	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			a.emitEvent("log", fmt.Sprintf("HTML保存を再試行します（%d/%d回目）: %s", retry+1, maxRetries, fileName))
			time.Sleep(2 * time.Second)
		}
		if err := writeMainHTML(filePath, []byte(mainHTML)); err == nil {
			return nil
		} else {
			lastErr = err
			a.emitEvent("log", fmt.Sprintf("HTML保存に失敗しました（%d/%d回目）: %s - %v", retry+1, maxRetries, fileName, err))
		}
	}
	return fmt.Errorf("HTML保存に%d回失敗しました: %s - %w", maxRetries, fileName, lastErr)
}

func writeMainHTML(filePath string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(filePath), ".narou-html-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, filePath)
}
