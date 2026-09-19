# OBS Network Monitor

[![Quality Gate](https://github.com/hazuki3417/obs-network-monitor/actions/workflows/quality-gate.yml/badge.svg?branch=develop)](https://github.com/hazuki3417/obs-network-monitor/actions/workflows/quality-gate.yml)
[![CodeQL](https://github.com/hazuki3417/obs-network-monitor/actions/workflows/codeql.yml/badge.svg?branch=develop)](https://github.com/hazuki3417/obs-network-monitor/actions/workflows/codeql.yml)

Windows PCが現在使用しているネットワークアダプターの通信量とインターネット品質を、OBS Browser Sourceへ表示するローカルモニターです。管理者権限、外部コマンド、外部テレメトリーを必要とせず、Web UIは実行ファイルへ埋め込まれています。

## 表示内容

- 使用中NICの現在の送信・受信速度と、直近60秒の通信量グラフ
- 最新の成功遅延、直近60回の平均・最小・最大遅延とジッター
- ICMP時のパケット損失率、HTTP時のリクエスト失敗率
- 現在の連続測定失敗回数
- 失敗測定を欠損として扱う直近60秒の遅延グラフ
- IPアドレスとホスト名を伏せた、最大6ノードのネットワーク経路

測定値をそのまま表示し、回線品質の良否は判定しません。

## 必要環境

- Windows 10またはWindows 11（64-bit）
- OBS StudioのBrowser Source、またはWebSocket対応ブラウザ
- Go 1.26以上（ソースからビルドする場合のみ）

## すぐに使う

### GitHub Release

1. GitHubの [Releases](https://github.com/hazuki3417/obs-network-monitor/releases) から使用するバージョンを開く。
2. `obs-network-monitor-windows-x64.zip` とチェックサムファイルをダウンロードする。
3. ZIPを展開し、`obs-network-monitor.exe` を通常ユーザー権限で実行する。
4. ブラウザで `http://127.0.0.1:8080/` を開き、ヘルスチェックが正常であることを確認する。
5. 終了するときは、同じページの **Stop application** を押す。

ZIPにはexe、README、設定例、MIT Licenseが含まれます。成果物は署名されていないため、Windowsが発行元を確認できない旨を表示する場合があります。必要に応じて同じReleaseの `.sha256` ファイルで整合性を確認してください。

### ソースから実行

```powershell
go run .
```

### ソースからビルド

```powershell
go test ./...
go build -trimpath -o obs-network-monitor.exe .
./obs-network-monitor.exe
```

ソースから通常ビルドした場合はコンソールが表示され、`Ctrl+C` でも終了できます。GitHub Releaseの配布版はコンソールを表示しないため、`http://127.0.0.1:8080/` の **Stop application** から終了します。起動時にブラウザを自動表示する機能はありません。

## リリース

正式版はGitHubのActions画面から公開できます。

1. **Prepare Release** を `develop` から実行し、`vMAJOR.MINOR.PATCH` を入力する。
2. 自動作成された `develop → main` の昇格PRを確認してマージする。
3. **Release** を `main` から実行し、同じバージョンを入力する。

Release Actionは `main` を再検証し、タグ、テスト済みWindows x64 ZIP、SHA-256、GitHub Releaseを作成します。従来どおり、`main` のコミットへバージョンタグをpushして起動することもできます。メンテナー向けの設定、画面操作、安全条件は [リリース手順](docs/releasing.md) を参照してください。

## 設定

設定は任意です。設定しない場合はGoogleの既定測定先を使用します。変更する場合は [config.example.json](config.example.json) を `config.json` という名前で **exeと同じフォルダー** にコピーします。

```json
{
  "icmpTarget": "8.8.8.8",
  "httpTarget": "https://www.google.com/generate_204",
  "traceroute": {
    "target": "",
    "intervalSeconds": 60,
    "maxNodes": 6
  }
}
```

| 項目 | 内容 |
| --- | --- |
| `icmpTarget` | IPv4アドレス、またはIPv4へ名前解決できるホスト名 |
| `httpTarget` | HTTPS URL |
| `traceroute.target` | Traceroute先。空文字の場合は `icmpTarget` を使用 |
| `traceroute.intervalSeconds` | 経路測定完了後から次の測定までの秒数（30–3600、既定60） |
| `traceroute.maxNodes` | OBSへ表示する最大ノード数（3–12、既定6） |

設定は起動時に1回だけ読み込みます。変更後はアプリを再起動してください。`icmpTarget` / `httpTarget` の空文字、未知のJSONフィールド、不正なURL、IPv6のみの測定先は起動エラーになります。`traceroute.target` の空文字だけは継承指定として有効です。エラー時は既定値へ自動的に戻しません。

## OBSへ追加

1. アプリを起動する。
2. ブラウザで `http://127.0.0.1:8080/` を開き、WebSocketとMonitor Streamが正常であることを確認する。
3. OBSの「ソース」で「ブラウザ」を追加する。
4. 表示したい情報に応じて、次のURLとサイズを設定する。
5. 必要に応じてOBS上で縮小・配置する。

OBS向け表示は背景、パネル、NIC情報を表示せず、透明背景へ重ねます。Latency、NIC Traffic、Routeは独立したBrowser Sourceとして配置します。ルート `/` は稼働確認と利用案内のページであり、OBS表示には使用しません。

| 表示内容 | URL | 推奨サイズ |
| --- | --- | --- |
| 遅延のみ | `http://127.0.0.1:8080/latency` | 480 x 270 |
| NIC通信量のみ | `http://127.0.0.1:8080/traffic` | 480 x 210 |
| 匿名化した経路 | `http://127.0.0.1:8080/route` | 480 x 300 |

各セクションの数値とグラフは、同じBrowser SourceとWebSocket接続のまま `parts` で切り替えられます。

| 表示内容 | クエリ例 |
| --- | --- |
| 数値とグラフ | `?parts=values,graph`（既定） |
| 数値のみ | `?parts=values` |
| グラフのみ | `?parts=graph` |

`parts` は `/latency` と `/traffic` で利用できます。たとえばNIC通信量のグラフだけを表示する場合は `http://127.0.0.1:8080/traffic?parts=graph` を使用します。未指定または有効な値がない場合は、空画面を避けるため数値とグラフの両方を表示します。`/route` は経路全体が1つの表示単位のため、`parts` には対応しません。

すべてのページは共通の `/ws` から同じ最新スナップショットを受信し、Browser Sourceを増やしてもバックエンドの測定回数は増えません。

OBSオーバーレイは通常のブラウザで開いた場合も同じ透明表示になります。WebSocketが切断された場合は最後の値を残し、2秒ごとに再接続します。

ネットワーク計測と数値更新は約1秒間隔ですが、グラフはサンプル時刻を基準にCanvasへ最大60fpsで描画し、60秒の時間窓を滑らかに移動します。数値は補間や更新エフェクトを加えず、受信した実測値へ即時更新します。グラフだけを2秒遅延させ、受信済みの曲線を左右の非表示領域まで描画してから中央の固定60秒だけを表示します。

## 測定の動作

通常は1秒ごとにWindows ICMP APIで測定します。ICMPが3回連続で失敗し、HTTPは成功した場合にHTTPへ切り替わります。HTTP測定中は30秒ごとにICMPを再確認し、回復していればICMPへ戻ります。

HTTP応答時間にはDNS、TCP、TLS、サーバー処理が含まれるため、ICMP RTTと同じ意味ではありません。方式を切り替える際は60サンプルの履歴をリセットし、異なる測定値を混在させません。

NICの送信・受信速度はWindowsが保持する累積バイト数を1秒ごとに比較して算出します。リンク速度はNICと接続先の理論上の速度、送受信速度は現在実際に流れている通信量であり、意味が異なります。初回取得、NIC変更、カウンターのリセット、取得失敗時は値なしになり、次の有効な差分から再開します。

TracerouteはWindows ICMP APIでTTLを1から最大30まで増やし、各ホップを1回、最大1秒で測定します。通常の遅延・通信量計測とは別に実行し、前回の測定完了から60秒後に次を開始するため、経路測定同士は重なりません。表示上はIPアドレスとホスト名を送信せず、`LOCAL`、集約した `HOP`、`TARGET` だけを表示します。隣接して応答したホップ間でRTTが20 ms以上増えた箇所を優先し、応答なしは弱い情報として `NO RESPONSE`、到達しなかった経路は `ROUTE INCOMPLETE` と表示します。各ホップのRTTは独立した往復時間であり、合計値ではありません。

## トラブルシューティング

| 症状 | 確認事項 |
| --- | --- |
| 起動直後に終了する | コンソールの `load config` エラーを確認し、`config.json` のJSON・測定先・HTTPS指定を修正する |
| `bind` エラーで起動できない | `127.0.0.1:8080` を使用している別プロセスを終了する |
| NICが未接続・状態不明になる | インターネット経路、VPN、指定したICMP測定先へのIPv4経路を確認する |
| HTTPへ切り替わる | ネットワークまたは測定先がICMP Echoを許可しているか確認する |
| 測定値が更新されない | ルートのヘルスチェックを確認し、OBSのURLが `/latency`、`/traffic`、`/route` のいずれか確認する |
| OBS向け表示が見切れる | Latencyは480 x 270、NIC Trafficは480 x 210、Routeは480 x 300にする |

設定、測定、NIC取得、Traceroute、WebSocket、HTTPサーバーのエラーは、exeと同じフォルダーの `logs/error.log` へ記録します。正常時は `logs` フォルダーを作成しません。ログが5 MiBへ達すると `error.previous.log` へ1世代だけローテーションします。

## MVPの制約

- Windows・IPv4のみ
- 現在の経路で選ばれるNICを1つだけ表示
- ポートは `127.0.0.1:8080` 固定で、外部PCからは接続不可
- 統計は直近60サンプル、表示は2秒遅延した直近60秒を対象とする。左右各2秒の非表示描画領域と1秒の時刻余裕を含む65秒以上の履歴をメモリ上に保持する（安全上限256件、永続化しない）
- 設定の動的再読み込みは非対応
- IPv6、帯域使用率、複数NIC同時表示、総合品質判定は対象外
- Tracerouteは最大30ホップ・各ホップ1プローブで、経路上のIPアドレスとホスト名はOBSへ配信しない
- HTTP測定の失敗率はパケット損失率ではない

## ライセンス

[MIT License](LICENSE)です。Copyright (c) 2026 hazuki3417.

## コントリビューションとセキュリティ

- 不具合報告と機能要望は [GitHub Issues](https://github.com/hazuki3417/obs-network-monitor/issues/new/choose) を利用してください。
- 開発手順とPull Requestの方針は [コントリビューションガイド](CONTRIBUTING.md) を参照してください。
- 脆弱性の詳細を公開Issueへ投稿せず、[セキュリティポリシー](SECURITY.md)に従って非公開で報告してください。
- メンテナー向けのGitHub設定は [Security and quality設定](docs/security-and-quality.md) を参照してください。

## 検証

`develop` 向けPull Request、`develop` へのpush、手動実行で、Windows Quality Gateが次を検証します。

- `go test ./...`
- Go、Devbox、開発文書のツールチェーンバージョン整合性
- Windows x64実行ファイルのビルド
- 設定なし・有効な設定での起動
- 不正設定の起動拒否
- 埋め込みUIのHTTP配信
- 複数のWebSocket接続へのスナップショット配信とJSON契約

NIC・経路切り替え、実ネットワークでのICMP / HTTP切り替え、OBS表示は [Windows MVP 手動確認チェックリスト](docs/windows-mvp-checklist.md) を使って確認してください。

詳細な仕様は [PCネットワークモニター MVP仕様](docs/network-monitor-mvp.md) を参照してください。
