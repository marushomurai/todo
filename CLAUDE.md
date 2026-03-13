# todo — マニャーナの法則 日次コミットメント装置

## 概要

CLIベースのクローズドリスト実装。タスク管理ではなく「今日はこれだけやる」を強制する。

## 技術スタック

- Go + cobra + modernc.org/sqlite (CGO不要)
- DB: ~/.config/todo/todo.db (TODO_DB環境変数でオーバーライド)

## コマンド (v0.1.0)

- `todo add "タスク"` — inbox追加
- `todo plan` — 朝の儀式: inbox→WILL DO選択→リスト閉鎖 (1日1回)
- `todo ls` — 今日のWILL DO表示
- `todo done <id>` — タスク完了 (ID限定)
- `todo review` — 夕方の儀式: 未完了→inbox戻し+統計

## グローバルフラグ

`--json`, `--quiet`, `--no-color`

## ビルド

```
make check   # go vet + go test
make build   # ローカルビルド
make install # go install
```

## Web UI

- Go + Templ + HTMX + Tailwind CSS v4 (standalone CLI, Node不要)
- Sortable.js (static/sortable.min.js, 45KB vendored)
- 2タブ構成: TODAY | INBOX
- TODAYページは4ステート: 朝の選択 → 実行中 → 振り返り → 完了
- `todo serve --port 3456` でHTTPサーバー起動

## デプロイ (OpenClaw = Mac Mini)

GitHub Releases にバイナリをアップロード → OpenClaw に以下を指示:

```
curl -L -o ~/Developer/Projects/todo/todo \
  "https://github.com/marushomurai/todo/releases/download/<VERSION>/todo-openclaw" \
  && chmod +x ~/Developer/Projects/todo/todo \
  && launchctl kickstart -k gui/$(id -u)/com.yuyanky.todo-bot \
  && launchctl kickstart -k gui/$(id -u)/com.yuyanky.todo-serve
```

- `com.yuyanky.todo-serve` — HTTPサーバー (port 3456)
- `com.yuyanky.todo-bot` — Telegram Bot
- MacBook Pro の localhost:3456 → Mac Mini へSSHトンネル

## マニャーナルール

- plan は1日1回 (--replan で再計画、v0.2.0)
- add は常にinboxのみ
- review 未完了はinbox戻し（翌朝再コミット）
- review忘れ → 翌朝plan時に自動carry over
