# NarouDownload

## About

narouDL([https://w.atwiki.jp/brown/](https://w.atwiki.jp/brown/))への多大なリスペクトをもって開発された小説家になろうのダウンローダー

## ダウンロード
[narou_download.zip](https://github.com/wishmaster127/narou_download/releases/download/1.1/narou_download_1.1.zip)

## TXT保存形式

標準では「読書用TXT（readable）」を保存します。ルビは `漢字（かんじ）` のような括弧表記になり、青空文庫の注記は取り除かれます。

青空文庫形式が必要な場合は「青空文庫TXT」を選択してください。両方の形式を同時に保存することもできます。

- 読書用TXT: `小説番号-話数.txt`（連結時は `all.txt`）
- 青空文庫TXT: `aozora-小説番号-話数.txt`（連結時は `aozora-all.txt`）

旧版の通常名TXTに青空文庫のルビ・注記が残っている場合は、再取得して読書用TXTを保存します。置き換える前に旧内容を `aozora-` 付きの名前へ退避し、同名の別内容がある場合は `-previous`（必要なら連番）を付けて残します。旧 `-readable.txt` はそのまま残ります。

保存済みの形式設定は引き継ぎます。以前の設定で青空文庫TXTが選択されている場合は、画面で読書用TXTへ切り替えてください。

## ※注
HTML形式での保存は未対応 

押し絵もダウンロードできません

## Live Development
`wails dev`


http://localhost:34115

## ビルド
`wails build`
