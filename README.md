# OBS Network Monitor

OBS Browser Source向けの最小ネットワークモニターです。

## 必要環境
- Windows
- Go 1.22以上（ビルド時のみ）

## 実行
```powershell
go mod tidy
go run .
```

ブラウザで `http://127.0.0.1:8080/` を開いて確認してください。

## OBSへの追加
1. ソース → `ブラウザ` を追加
2. URL: `http://127.0.0.1:8080/`
3. 幅/高さは任意（例: 400 x 200）

## exeを作る
```powershell
go build -o obs-network-monitor.exe .
```

HTML/CSS/JavaScriptはGoの `embed` によりexeへ埋め込まれます。

## Quality Gate

`develop` 向けのPull Requestと `develop` へのpushでは、GitHub ActionsのWindows runnerで次を確認します。

- `go test ./...`
- `obs-network-monitor.exe` のビルド

GitHub Actionsの画面から手動実行することもできます。

## MVP仕様

PCネットワークモニターの合意済み仕様は [docs/network-monitor-mvp.md](docs/network-monitor-mvp.md) を参照してください。

## 現在の表示内容
- ONLINE / OFFLINE
- HTTPSリクエストの応答時間（簡易Latency）
- 最終更新時刻

※ LatencyはICMP Pingではなく `https://www.google.com/generate_204` へのHTTP応答時間です。今後、NIC送受信量やICMP Ping、packet loss、OBS統計などを追加できます。
