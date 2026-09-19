# Security and quality設定

この文書は、リポジトリへファイルをマージした後に、GitHubの管理画面で行う設定をまとめたものです。設定画面の名称はGitHub側の更新で変わる場合があります。

## 1. セキュリティ機能

リポジトリの **Settings** → **Advanced Security** で、利用可能な次の項目を有効にします。

- Dependency graph
- Dependabot alerts
- Dependabot security updates
- Secret scanning
- Push protection
- Private vulnerability reporting

Dependabot version updatesは `.github/dependabot.yml` のマージ後に有効になります。Go ModulesとGitHub Actionsを毎週確認し、同じエコシステムの更新を1つのPull Requestへまとめます。

Code scanningは `.github/workflows/codeql.yml` のマージ後にAdvanced setupとして動作します。GitHub画面からDefault setupを重ねて有効にしません。

## 2. 初回実行の確認

ファイルを `develop` へマージした後、次を確認します。

1. **Actions** → **CodeQL** を開く。
2. `develop` の **Analyze Go on Windows** が成功していることを確認する。
3. **Security and quality** → **Code scanning** に設定エラーがないことを確認する。
4. Dependabotが `.github/dependabot.yml` を正常な設定として認識していることを確認する。

CodeQLはWindows固有のGoコードをWindows上でビルドしてから解析します。既存のQuality Gateとは目的が異なるため、両方を維持します。

## 3. ブランチRuleset

CodeQLの初回成功後、**Settings** → **Rules** → **Rulesets** からブランチRulesetを作成します。

### 対象

- `develop`
- `main`

最初は1つのRulesetで両方を対象にして構いません。運用差が必要になった時点で分割します。

### 有効にするルール

- Restrict deletions
- Block force pushes
- Require a pull request before merging
- Require status checks to pass
- Require branches to be up to date before merging

必須チェックには、実際に一度成功して選択肢へ現れた次のチェックを指定します。

- **Windows build and smoke test**
- **Analyze Go on Windows**

個人開発で自己承認が必要になることを避けるため、Required approvalsは `0` のままにします。Require review from Code Ownersも有効にしません。

Prepare Releaseが作成する `develop → main` のPull Requestでも、同じコミットに対する2つのチェックが成功していることを確認してからマージします。

## 4. 今回変更しない設定

- `v*`タグの作成制限: Release Actionによるタグ作成を妨げる可能性があるため、専用のバイパス設計を行うまで追加しない。
- デフォルトブランチ: 現在の `develop` を維持する。`main` への変更は、Dependabotの送信先とPull Requestの既定送信先を含めて別途検討する。
- 必須レビュー: 外部メンテナーが参加するまで要求しない。

## 5. リポジトリ情報

リポジトリの **About** へ次を設定します。

- Description: `Windowsのネットワーク状態をOBS Browser Sourceへ表示するローカルモニター`
- Topics: `obs-studio`, `network-monitoring`, `windows`, `golang`, `websocket`

不要な機能を増やさず、利用目的と対応環境を検索結果から判断できる情報だけを設定します。
