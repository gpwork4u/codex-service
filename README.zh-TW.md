# codex-service

[English](README.md) | 繁體中文

一個輕量的 Go proxy 服務，透過 ChatGPT OAuth 認證 OpenAI Codex，對外提供 OpenAI API 相容的端點。讓你的 ChatGPT Plus/Pro 訂閱驅動任何支援 OpenAI API 的工具。

> **免責聲明**：本專案使用與官方 [OpenAI Codex CLI](https://github.com/openai/codex) 相同的公開 OAuth 端點和 Codex API，但不保證符合 OpenAI 的服務條款。**使用風險自負。** 作者不對因使用本工具而導致的帳號封禁或其他後果承擔任何責任。

## 功能

- **ChatGPT OAuth 認證** — Device code flow，不需要 API key
- **OpenAI API 相容** — 任何支援 OpenAI API 的工具都能直接接（Cursor、aider、Continue 等）
- **Chat Completions → Responses API 轉換** — 自動轉換請求格式
- **SSE 串流** — 完整串流支援，含格式轉換
- **自動 Token 刷新** — 自動處理 OAuth token 生命週期
- **零外部依賴** — 純 Go 標準函式庫

## 快速開始

### 編譯 & 登入

```bash
go build -o codex-service .
./codex-service login
```

依照指示到 `auth.openai.com/codex/device` 完成 ChatGPT 帳號認證。

### 啟動服務

```bash
./codex-service
```

服務預設啟動於 `http://localhost:8787`。

### Docker

```bash
# 先在 host 上登入
go run . login

# 用 Docker Compose 啟動
docker compose up -d --build
```

## API 端點

| 端點 | 方法 | 說明 |
|------|------|------|
| `/v1/chat/completions` | POST | OpenAI Chat Completions 相容端點 |
| `/v1/responses` | POST | OpenAI Responses API 直接轉發 |
| `/v1/models` | GET | 列出可用的 Codex 模型 |
| `/health` | GET | 健康檢查 |

## 可用模型

- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.3-codex`
- `gpt-5.2-codex`
- `gpt-5.2`
- `gpt-5.1-codex`
- `gpt-5.1-codex-max`
- `gpt-5.1-codex-mini`

## 使用範例

### curl

```bash
# 非串流
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {"role": "system", "content": "你是一位有用的助手。"},
      {"role": "user", "content": "你好"}
    ]
  }'

# 串流
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'
```

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8787/v1", api_key="anything")
resp = client.chat.completions.create(
    model="gpt-5.4",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)
```

### 工具設定

任何 OpenAI 相容工具（Cursor、Continue、aider 等）：

- **Base URL**：`http://localhost:8787/v1`
- **API Key**：任意值（或設定 `CODEX_LOCAL_AUTH` 進行驗證）
- **Model**：`gpt-5.4`（或上方列表中的任何模型）

## 支援的參數

送到 Codex API 的參數：

| Chat Completions 參數 | Codex 對應 | 說明 |
|---|---|---|
| `messages[role=system]` | `instructions` | 必填，預設為 "You are a helpful assistant." |
| `messages[role=user/assistant]` | `input` | 對話訊息 |
| `model` | `model` | |
| `tools` | `tools` | |
| `tool_choice` | `tool_choice` | |
| `response_format` | `text` | |

靜默忽略的參數（Codex Responses API 不支援）：

`temperature`、`top_p`、`max_tokens`、`max_completion_tokens`、`stop`、`frequency_penalty`、`presence_penalty`、`seed`、`user`

Proxy 自動設定的參數：

| 參數 | 值 |
|---|---|
| `stream` | `true`（Codex 要求串流） |
| `store` | `false` |
| `reasoning.effort` | `"medium"` |
| `reasoning.summary` | `"auto"` |

## 環境變數

| 環境變數 | 預設值 | 說明 |
|---|---|---|
| `CODEX_LISTEN_ADDR` | `:8787` | 監聽地址 |
| `CODEX_DATA_DIR` | `~/.codex-service` | 憑證存放目錄 |
| `CODEX_LOCAL_AUTH` | _（無）_ | 可選，incoming 請求的 bearer token 驗證 |

## 運作原理

```
Client（Cursor、aider 等）
  │ POST /v1/chat/completions
  ▼
codex-service（localhost:8787）
  │ 1. 提取 system messages → instructions
  │ 2. 轉換 Chat Completions → Responses 格式
  │ 3. 注入 OAuth token + ChatGPT-Account-Id header
  ▼
chatgpt.com/backend-api/codex/responses
  │ SSE 串流回應
  ▼
codex-service
  │ 轉換 Responses → Chat Completions 格式
  ▼
Client 收到 OpenAI 相容的回應
```

## 免責聲明

本專案按原樣提供，僅供教育和個人使用。本專案使用與官方 OpenAI Codex CLI 相同的公開 OAuth 端點，但與 OpenAI 無任何關聯或背書。作者不對因使用本工具而導致的帳號封禁、資料遺失或其他後果承擔任何責任。使用風險自負。

## 授權

MIT
