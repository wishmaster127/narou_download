import {
  TextInput,
  Checkbox,
  Textarea,
  Button,
  Select,
  Card,
  Group,
  Stack,
  Text,
  Grid,
  Progress
} from '@mantine/core'
import { useState, useEffect, useRef } from 'react'
import { 
  DownloadNovel, 
  SelectFolder, 
  OpenFolder, 
  SetAlwaysOnTop,
  SaveSettings,
  LoadSettings,
  Quit,
  GetTitle,
} from '../../wailsjs/go/main/App'

export default function NarouDownload() {
  const [log, setLog] = useState('')
  const logTextareaRef = useRef(null)
  const [encoding, setEncoding] = useState('UTF-8')
  const [lineEnding, setLineEnding] = useState('CR+LF')
  const [progress, setProgress] = useState(0)
  const [savePath, setSavePath] = useState('')
  const [url, setUrl] = useState('')
  const [showInFront, setShowInFront] = useState(false)
  const [createHtml, setCreateHtml] = useState(true)
  const [createTxt, setCreateTxt] = useState(true)
  const [createCombined, setCreateCombined] = useState(false)
  const [title, setTitle] = useState('')
  const [progressText, setProgressText] = useState('')
  const [isDownloading, setIsDownloading] = useState(false)

  // 設定の読み込み
  useEffect(() => {
    const loadSettings = async () => {
      try {
        const settings = await LoadSettings()
        setUrl(settings.url || '')
        setSavePath(settings.savePath || '')
        setEncoding(settings.encoding || 'UTF-8')
        setLineEnding(settings.lineEnding || 'CR+LF')
        setCreateHtml(settings.createHtml ?? true)
        setCreateTxt(settings.createTxt ?? true)
        setCreateCombined(settings.createCombined ?? false)
        setShowInFront(settings.showInFront ?? false)
      } catch (error) {
        console.error('設定の読み込み中にエラーが発生しました:', error)
      }
    }
    loadSettings()

    // イベントリスナーの登録
    const progressUnsubscribe = window.runtime.EventsOn("progress", (value) => {
      setProgress(value)
    })

    const logUnsubscribe = window.runtime.EventsOn("log", (message) => {
      setLog(prev => prev + '\n' + message)
    })
    
    const progressTextUnsubscribe = window.runtime.EventsOn("progressText", (text) => {
      setProgressText(text)
    })

    // クリーンアップ関数
    return () => {
      progressUnsubscribe()
      logUnsubscribe()
      if (progressTextUnsubscribe) progressTextUnsubscribe()
    }
  }, [])

  const handleDownload = async () => {
    // 入力バリデーション
    if (!url) {
      setLog('エラー: URLが入力されていません')
      return
    }
    
    if (!url.includes('syosetu.com')) {
      setLog('エラー: 小説家になろう(ncode.syosetu.com)またはノクターンノベルズ(novel18.syosetu.com)のURLを入力してください')
      return
    }
    
    if (!createHtml && !createTxt) {
      setLog('エラー: HTMLまたはTXTのどちらかを選択してください')
      return
    }

    try {
      setIsDownloading(true)
      setLog('ダウンロードを開始します...')
      setProgress(0)
      setProgressText('初期化中...')
      
      // タイトルを取得
      try {
        const novelTitle = await GetTitle(url)
        setTitle(novelTitle)
      } catch (error) {
        console.error('タイトル取得エラー:', error)
        setTitle('タイトルを取得できませんでした')
      }
      
      const options = {
        encoding,
        lineEnding,
        createHtml,
        createTxt,
        createCombined,
        showInFront
      }
      await DownloadNovel(url, savePath, options)
      setProgressText('完了')
    } catch (error) {
      console.error('ダウンロード中にエラーが発生しました:', error)
      setLog(prev => prev + '\nエラー: ダウンロードに失敗しました - ' + error.message)
      setProgress(0)
      setProgressText('エラー')
      setTitle('')
    } finally {
      setIsDownloading(false)
    }
  }

  const handleSelectFolder = async () => {
    try {
      const path = await SelectFolder()
      setSavePath(path)
    } catch (error) {
      console.error('フォルダ選択中にエラーが発生しました:', error)
      setLog(prev => prev + '\nエラー: フォルダ選択に失敗しました - ' + error.message)
    }
  }

  const handleOpenFolder = async () => {
    if (!savePath) {
      setLog(prev => prev + '\nエラー: 保存先パスが指定されていません')
      return
    }
    try {
      await OpenFolder(savePath)
    } catch (error) {
      console.error('フォルダを開く際にエラーが発生しました:', error)
      setLog(prev => prev + '\nエラー: フォルダを開けませんでした - ' + error.message)
    }
  }

  const handleShowInFrontChange = async (event) => {
    const checked = event.currentTarget.checked
    setShowInFront(checked)
    try {
      await SetAlwaysOnTop(checked)
    } catch (error) {
      console.error('常に手前に表示の設定中にエラーが発生しました:', error)
      setLog(prev => prev + '\nエラー: 常に手前に表示の設定に失敗しました - ' + error.message)
    }
  }

  const handleExit = async () => {
    try {
      await Quit()
    } catch (error) {
      console.error('終了処理中にエラーが発生しました:', error)
    }
  }

  const handleUrlChange = (e) => {
    const newUrl = e.target.value
    setUrl(newUrl)
    
    // URLが変更されたらタイトルをクリア
    setTitle('')
  }

  useEffect(() => {
    const syncSettings = async () => {
      try {
        const settings = {
          url,
          savePath,
          encoding,
          lineEnding,
          createHtml,
          createTxt,
          createCombined,
          showInFront
        }
        await SaveSettings(settings)
      } catch (error) {
        console.error('設定の同期中にエラーが発生しました:', error)
      }
    }
  
    syncSettings()
  }, [url, savePath, encoding, lineEnding, createHtml, createTxt, createCombined, showInFront])

  // ログが更新されたときに自動スクロール
  useEffect(() => {
    if (logTextareaRef.current) {
      logTextareaRef.current.scrollTop = logTextareaRef.current.scrollHeight
    }
  }, [log])

  return (
    <Card
      padding="md"
      radius={0}
      withBorder={false}
      style={{ width: '100vw', height: '100vh' }}
    >
      <Stack spacing="md">
        <Grid>
          <Grid.Col span={2}>タイトル</Grid.Col>
          <Grid.Col span={10}>
            <Text 
              align="left" 
              style={{ 
                fontWeight: title ? 'bold' : 'normal', 
                color: title ? '#228be6' : '#868e96',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                maxWidth: '100%'
              }}
              title={title} // ホバー時に全文表示
            >
              {title || 'ダウンロード時に小説のタイトルが表示されます'}
            </Text>
          </Grid.Col>

          <Grid.Col span={2}>アドレス</Grid.Col>
          <Grid.Col span={10}>
            <TextInput 
              value={url}
              onChange={handleUrlChange}
              placeholder="小説のURLを入力してください"
            />
          </Grid.Col>

          <Grid.Col span={2}>保存先</Grid.Col>
          <Grid.Col span={10}>
            <Group spacing="xs">
              <TextInput 
                placeholder="保存先のパス" 
                style={{ flex: 1 }}
                value={savePath}
                onChange={(e) => setSavePath(e.target.value)}
              />
              <Button 
                variant="default"
                onClick={handleSelectFolder}
              >
                参照
              </Button>
              <Button 
                variant="default"
                onClick={handleOpenFolder}
              >
                開く
              </Button>
            </Group>
          </Grid.Col>

          <Grid.Col span={10} offset={2}>
            <Group>
              {/* <Checkbox 
                checked={createHtml}
                onChange={(event) => setCreateHtml(event.currentTarget.checked)}
                label="HTML" 
              /> */}
              <Checkbox 
                checked={createTxt}
                onChange={(event) => setCreateTxt(event.currentTarget.checked)}
                label="TXT" 
              />
              <Select
                value={encoding}
                onChange={setEncoding}
                data={['UTF-8', 'UTF-16LE', 'Shift-JIS']}
                style={{ flex: 1 }}
              />
              <Select
                value={lineEnding}
                onChange={setLineEnding}
                data={['CR+LF', 'LF', 'CR']}
                style={{ flex: 1 }}
              />
            </Group>
          </Grid.Col>
        </Grid>

        <Textarea 
          ref={logTextareaRef}
          value={log} 
          readOnly 
          rows={10}
          style={{ overflowY: 'auto' }}
        />

        <div>
          <Group position="apart" mb={5}>
            <Text size="sm" weight={500}>進捗: {progress}%</Text>
            <Text size="sm" c="dimmed">{progressText}</Text>
          </Group>
          <Progress radius="sm" size="xl" value={progress} />
        </div>

        <Group position="right" align="flex-end">
          <Stack>
            <Checkbox 
              label="連結ファイルの作成"
              checked={createCombined}
              onChange={(event) => setCreateCombined(event.currentTarget.checked)}
            />
            <Checkbox 
              label="手前に表示" 
              checked={showInFront}
              onChange={handleShowInFrontChange}
            />
          </Stack>

          <Button ml={50} onClick={handleDownload} disabled={isDownloading || !url}>
            {isDownloading ? 'ダウンロード中...' : 'ダウンロード'}
          </Button>
          <Button variant="default" onClick={handleExit}>終了</Button>
        </Group>
      </Stack>
    </Card>
  )
}
