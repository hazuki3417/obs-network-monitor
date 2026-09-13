# OBS Network Monitor

Windows PCが現在使用しているネットワークアダプターとインターネット品質を、OBS Browser Sourceまたは通常のブラウザへ表示するローカルモニターです。管理者権限、外部コマンド、外部テレメトリーを必要とせず、Web UIは実行ファイルへ埋め込まれています。

## 表示内容

- 現在の経路で使われるNIC名、接続状態、送受信リンク速度
- 使用中NICの現在の送信・受信速度と、最大60サンプルの通信量グラフ
- ICMPまたはHTTPによる測定方式と測定先
- 最新の成功遅延、直近60回の平均・最小・最大遅延とジッター
- ICMP時のパケット損失率、HTTP時のリクエスト失敗率
- 現在の連続測定失敗回数
- 失敗測定を欠損として扱う最大60サンプルの遅延グラフ
- WebSocketの接続・再接続状態

測定値をそのまま表示し、回線品質の良否は判定しません。

## 必要環境

- Windows 10またはWindows 11（64-bit）
- OBS StudioのBrowser Source、またはWebSocket対応ブラウザ
- Go 1.22以上（ソースからビルドする場合のみ）

## すぐに使う

### GitHub Release

1. GitHubの [Releases](https://github.com/hazuki3417/obs-network-monitor/releases) から使用するバージョンを開く。
2. `obs-network-monitor-windows-x64.zip` とチェックサムファイルをダウンロードする。
3. ZIPを展開し、`obs-network-monitor.exe` を通常ユーザー権限で実行する。
4. ブラウザで `http://127.0.0.1:8080/` を開く。

ZIPにはexe、README、設定例が含まれます。成果物は署名されていないため、Windowsが発行元を確認できない旨を表示する場合があります。必要に応じて同じReleaseの `.sha256` ファイルで整合性を確認してください。

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

起動中はコンソールを閉じないでください。終了するときは `Ctrl+C` を押します。

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
  "httpTarget": "https://www.google.com/generate_204"
}
```

| 項目 | 内容 |
| --- | --- |
| `icmpTarget` | IPv4アドレス、またはIPv4へ名前解決できるホスト名 |
| `httpTarget` | HTTPS URL |

設定は起動時に1回だけ読み込みます。変更後はアプリを再起動してください。空文字、未知のJSONフィールド、不正なURL、IPv6のみの測定先は起動エラーになります。エラー時は既定値へ自動的に戻しません。

## OBSへ追加

1. アプリを起動する。
2. OBSの「ソース」で「ブラウザ」を追加する。
3. URLを `http://127.0.0.1:8080/?view=overlay` にする。
4. 幅と高さをそれぞれ `480` にする。
5. 必要に応じてOBS上で縮小・配置する。

OBS向け表示は背景、パネル、NIC情報を表示せず、遅延とNIC通信量を透明背景へ重ねます。セクションを個別のBrowser Sourceとして配置する場合は、次のURLとサイズを使用できます。

| 表示内容 | URL | 推奨サイズ |
| --- | --- | --- |
| 遅延とNIC通信量 | `http://127.0.0.1:8080/?view=overlay` | 480 x 480 |
| 遅延のみ | `http://127.0.0.1:8080/?view=overlay&sections=latency` | 480 x 270 |
| NIC通信量のみ | `http://127.0.0.1:8080/?view=overlay&sections=traffic` | 480 x 210 |

通常のWebダッシュボードは `http://127.0.0.1:8080/` で引き続き利用できます。WebSocketが切断されると通常表示では最後の値を残して `再接続中` と表示し、2秒ごとに再接続します。

## 測定の動作

通常は1秒ごとにWindows ICMP APIで測定します。ICMPが3回連続で失敗し、HTTPは成功した場合にHTTPへ切り替わります。HTTP測定中は30秒ごとにICMPを再確認し、回復していればICMPへ戻ります。

HTTP応答時間にはDNS、TCP、TLS、サーバー処理が含まれるため、ICMP RTTと同じ意味ではありません。方式を切り替える際は60サンプルの履歴をリセットし、異なる測定値を混在させません。

NICの送信・受信速度はWindowsが保持する累積バイト数を1秒ごとに比較して算出します。リンク速度はNICと接続先の理論上の速度、送受信速度は現在実際に流れている通信量であり、意味が異なります。初回取得、NIC変更、カウンターのリセット、取得失敗時は値なしになり、次の有効な差分から再開します。

## トラブルシューティング

| 症状 | 確認事項 |
| --- | --- |
| 起動直後に終了する | コンソールの `load config` エラーを確認し、`config.json` のJSON・測定先・HTTPS指定を修正する |
| `bind` エラーで起動できない | `127.0.0.1:8080` を使用している別プロセスを終了する |
| NICが未接続・状態不明になる | インターネット経路、VPN、指定したICMP測定先へのIPv4経路を確認する |
| HTTPへ切り替わる | ネットワークまたは測定先がICMP Echoを許可しているか確認する |
| `再接続中` のままになる | exeが起動中か、ブラウザURLが `http://127.0.0.1:8080/` か確認する |
| OBS向け表示が見切れる | URLに `?view=overlay` を付け、Browser Sourceを480 x 480にする |

測定・NIC取得の一時エラーではプロセスを終了せず、コンソールへ原因を記録して継続します。

## MVPの制約

- Windows・IPv4のみ
- 現在の経路で選ばれるNICを1つだけ表示
- ポートは `127.0.0.1:8080` 固定で、外部PCからは接続不可
- 直近60サンプルのみ保持し、測定結果は永続化しない
- 設定の動的再読み込みは非対応
- IPv6、帯域使用率、複数NIC同時表示、総合品質判定は対象外
- HTTP測定の失敗率はパケット損失率ではない

## 検証

`develop` 向けPull Request、`develop` へのpush、手動実行で、Windows Quality Gateが次を検証します。

- `go test ./...`
- Windows x64実行ファイルのビルド
- 設定なし・有効な設定での起動
- 不正設定の起動拒否
- 埋め込みUIのHTTP配信
- 2つのWebSocket接続へのスナップショット配信とJSON契約

NIC・経路切り替え、実ネットワークでのICMP / HTTP切り替え、OBS表示は [Windows MVP 手動確認チェックリスト](docs/windows-mvp-checklist.md) を使って確認してください。

詳細な仕様は [PCネットワークモニター MVP仕様](docs/network-monitor-mvp.md) を参照してください。
