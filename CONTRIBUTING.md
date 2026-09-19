# コントリビューションガイド

OBS Network Monitorへの改善提案を歓迎します。変更を始める前に、既存のIssueとPull Requestに同じ内容がないか確認してください。大きな仕様変更は、実装前にIssueで目的と範囲を相談してください。

## 開発対象

現在の基本方針は次のとおりです。

- Windows 10 / 11（64-bit）
- IPv4
- 現在使用中のNICを1つだけ監視
- OBS Browser Source向けのローカルWeb UI
- 管理者権限、外部コマンド、外部テレメトリーを使用しない

この範囲を変更する提案では、一般利用者への効果、UIへの影響、保守コストをIssueへ記載してください。

## ブランチとPull Request

- 通常の変更は `develop` を対象にPull Requestを作成します。
- `main` は公開版の昇格先です。通常の機能・修正PRを直接送らないでください。
- 1つのPull Requestには、関連する1つの目的だけを含めてください。
- ユーザー向けの動作や設定を変更した場合はREADMEまたは仕様書も更新してください。

## 開発と検証

Go 1.26以上とNode.jsを使用します。Node.jsは埋め込みWeb UIのテストにだけ使用し、実行ファイルの動作には不要です。

```powershell
go test ./...
node .github/scripts/toolchain-version-test.js
node .github/scripts/web-ui-test.js
go build -trimpath -o obs-network-monitor.exe .
```

Windows固有API、実行ファイル、WebSocket契約は、Pull Requestで起動するWindows Quality Gateでも検証されます。UIを変更した場合は、通常ブラウザだけでなくOBS Browser Sourceでも推奨サイズの表示を確認してください。

## コードとコミット

- Goコードは `gofmt` を適用してください。
- 新しい動作には、可能な範囲で単体テストまたはUIテストを追加してください。
- ログ、スクリーンショット、テストデータへ個人情報、IPアドレス、トークンを含めないでください。
- コミットメッセージは、変更の目的が分かる短い文にしてください。

## セキュリティ問題

脆弱性の詳細は公開IssueやPull Requestへ記載せず、[セキュリティポリシー](SECURITY.md)に従って非公開で報告してください。
