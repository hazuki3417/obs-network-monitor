# リリース手順

OBS Network Monitorの正式リリースは、`main` のコミットに付けたバージョンタグからGitHub Actionsで作成します。

## バージョン

タグは `vMAJOR.MINOR.PATCH` 形式のSemantic Versioningを使用します。

- 初回MVPリリース: `v0.1.0`
- 後方互換のある修正: PATCHを増やす
- 後方互換のある機能追加: MINORを増やす
- 互換性を壊す変更: MAJORを増やす

プレリリース識別子（例: `v0.2.0-beta.1`）は現在のActionでは受け付けません。

## 公開手順

1. `develop` から `main` へのPull Requestを作成する。
2. Quality Gateとレビューが完了したことを確認してマージする。
3. ローカルの `main` を最新化する。
4. マージコミットへ注釈付きタグを作成し、タグをpushする。

```powershell
git fetch origin
git switch main
git pull --ff-only origin main
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

タグpush後、GitHub Actionsの **Release** workflowが次を実行します。

1. タグ形式と、対象コミットが `main` の履歴に含まれることを検証
2. `go test ./...`
3. Windows x64実行ファイルのビルド
4. Windows結合スモークテスト
5. 配布ZIPとSHA-256チェックサムの生成
6. 自動生成したリリースノート付きGitHub Releaseの作成

## 公開物

- `obs-network-monitor-windows-x64.zip`
- `obs-network-monitor-windows-x64.zip.sha256`

ZIPには次のファイルが含まれます。

- `obs-network-monitor.exe`
- `config.example.json`
- `README.md`

成果物はコード署名されていません。チェックサムはダウンロード破損や意図しない差し替えの確認に使用できます。

```powershell
Get-FileHash .\obs-network-monitor-windows-x64.zip -Algorithm SHA256
Get-Content .\obs-network-monitor-windows-x64.zip.sha256
```

## 再実行と失敗時

同じタグのworkflowを再実行した場合、既存Releaseを重複作成せず、ZIPとチェックサムを検証済みの新しい成果物で置き換えます。

テスト、ビルド、スモークテストのいずれかが失敗した場合、Releaseの作成・更新処理へ進みません。原因を修正した新しいコミットを `develop` から `main` へ反映し、バージョンを増やした新しいタグで公開してください。

誤ったコミットへタグをpushした場合は、そのタグを使い回さず、GitHub上のReleaseとタグの状態を確認してから取り消します。公開済みバージョンを別コミットへ付け替えないでください。
