# リリース手順

OBS Network Monitorの正式リリースは、GitHubのActions画面から昇格PRの作成と公開を行います。ローカルGitや長期Personal Access Tokenは必要ありません。

## バージョン

タグは `vMAJOR.MINOR.PATCH` 形式のSemantic Versioningを使用します。

- 初回MVPリリース: `v0.1.0`
- 後方互換のある修正: PATCHを増やす
- 後方互換のある機能追加: MINORを増やす
- 互換性を壊す変更: MAJORを増やす

プレリリース識別子（例: `v0.2.0-beta.1`）は現在のActionでは受け付けません。

## 初回設定

Prepare ReleaseがPull Requestを作成するには、リポジトリの次の設定を有効にする。

1. GitHubリポジトリの **Settings** を開く。
2. **Actions** → **General** を開く。
3. **Workflow permissions** にある **Allow GitHub Actions to create and approve pull requests** を有効にする。

Workflowは必要なジョブだけに `pull-requests: write` または `contents: write` を指定する。リポジトリへ長期トークンを登録しない。

## GitHub画面からの公開手順

### 1. 昇格PRを準備する

1. GitHubリポジトリの **Actions** を開く。
2. 左側から **Prepare Release** を選ぶ。
3. **Run workflow** を開き、ブランチに `develop` を選ぶ。
4. `version` に、例として `v0.2.0` を入力して実行する。

Prepare Releaseは次を検証する。

- バージョンが `vMAJOR.MINOR.PATCH` 形式である
- 同名タグが存在しない
- `develop` に `main` へ反映するコミットがある
- 既存の `develop → main` PRがない
- Goテスト、Web UIテスト、Windows x64ビルド、Windows結合スモークテストが成功する
- 検証中に `develop` の先端が変わっていない

すべて成功すると、`Release vX.Y.Z: developをmainへ昇格` というPull Requestを作成する。

### 2. 昇格PRをマージする

作成されたPull Requestの差分と、本文に記録されたPrepare Release内のQuality Gateを確認して `main` へマージする。マージはActionでは自動化せず、人が明示的に承認する。`GITHUB_TOKEN`が作成したPull Requestは別workflowを自動起動しないため、昇格前検証はPrepare Release自身が再利用可能なQuality Gateを呼び出して実施する。人がマージした後は、`main` へのpushを対象とする通常のQuality Gateも起動する。

### 3. Releaseを公開する

1. **Actions** から **Release** を選ぶ。
2. **Run workflow** を開き、ブランチに必ず `main` を選ぶ。
3. Prepare Releaseと同じ `version` を入力して実行する。

Releaseは選択された `main` のコミットについて次を実行する。

1. バージョン形式、同名タグがないこと、対象コミットが `main` に含まれることを検証
2. `go test ./...`
3. Web UIテスト
4. Windows x64実行ファイルのビルド
5. Windows結合スモークテスト
6. 配布ZIPとSHA-256チェックサムの生成
7. `main` の検証済みコミットへのタグ作成
8. 自動生成したリリースノート付きGitHub Releaseの作成

テストまたはビルドが失敗した場合はタグとReleaseを作成しない。

## タグpushによる公開

既存運用との互換性のため、`main` のコミットへ `vMAJOR.MINOR.PATCH` タグをpushしてReleaseを起動する方法も維持する。

```powershell
git fetch origin
git switch main
git pull --ff-only origin main
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

タグ起動でも、タグの対象が `main` に含まれることを検証してから同じテストと公開処理を行う。

## 公開物

- `obs-network-monitor-windows-x64.zip`
- `obs-network-monitor-windows-x64.zip.sha256`

ZIPには次のファイルが含まれます。

- `obs-network-monitor.exe`
- `config.example.json`
- `README.md`
- `LICENSE`
- `docs/images/*.png`（README用の画面イメージ）

成果物はコード署名されていません。チェックサムはダウンロード破損や意図しない差し替えの確認に使用できます。

```powershell
Get-FileHash .\obs-network-monitor-windows-x64.zip -Algorithm SHA256
Get-Content .\obs-network-monitor-windows-x64.zip.sha256
```

## 再実行と失敗時

タグpushで起動した同じworkflowを再実行した場合、既存Releaseを重複作成せず、ZIPとチェックサムを検証済みの新しい成果物で置き換える。Actions画面からの手動公開は既存タグを拒否するため、公開済みバージョンの再実行には該当する過去のworkflowの **Re-run jobs** を使用する。

Prepare ReleaseがPR作成権限エラーになった場合は、初回設定のチェックボックスを確認する。テスト、ビルド、スモークテストのいずれかが失敗した場合、PR作成またはRelease公開へ進まない。原因を `develop` で修正し、Prepare Releaseからやり直す。

誤ったコミットへタグをpushした場合は、そのタグを使い回さず、GitHub上のReleaseとタグの状態を確認してから取り消す。公開済みバージョンを別コミットへ付け替えない。
