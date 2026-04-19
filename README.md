# codex-service

A lightweight Go proxy that authenticates with OpenAI Codex via ChatGPT OAuth and exposes OpenAI-compatible API endpoints. Use your ChatGPT Plus/Pro subscription to power any tool that supports the OpenAI API.

> **Disclaimer**: This project uses the same public OAuth endpoints and Codex API as the official [OpenAI Codex CLI](https://github.com/openai/codex). However, there is no guarantee that this usage complies with OpenAI's Terms of Service. **Use at your own risk.** The author assumes no responsibility for any account suspension or other consequences resulting from the use of this tool.

## Features

- **ChatGPT OAuth authentication** — Device code flow, no API key needed
- **OpenAI API compatible** — Works with any tool that supports the OpenAI API (Cursor, aider, Continue, etc.)
- **Chat Completions → Responses API translation** — Automatically converts between formats
- **SSE streaming** — Full streaming support with format translation
- **Auto token refresh** — Handles OAuth token lifecycle automatically
- **Zero external dependencies** — Pure Go stdlib

## Quick Start

### Build & Login

```bash
go build -o codex-service .
./codex-service login
```

Follow the instructions to authenticate with your ChatGPT account at `auth.openai.com/codex/device`.

### Run

```bash
./codex-service
```

The service starts on `http://localhost:8787` by default.

### Docker

```bash
# Login first (on host)
go run . login

# Run with Docker Compose
docker compose up -d --build
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/chat/completions` | POST | OpenAI Chat Completions compatible endpoint |
| `/v1/responses` | POST | OpenAI Responses API passthrough |
| `/v1/models` | GET | List available Codex models |
| `/health` | GET | Health check |

## Available Models

- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.3-codex`
- `gpt-5.2-codex`
- `gpt-5.2`
- `gpt-5.1-codex`
- `gpt-5.1-codex-max`
- `gpt-5.1-codex-mini`

## Usage Examples

### curl

```bash
# Non-streaming
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "hello"}
    ]
  }'

# Streaming
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [{"role": "user", "content": "hello"}],
    "stream": true
  }'
```

### Python (OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8787/v1", api_key="anything")
resp = client.chat.completions.create(
    model="gpt-5.4",
    messages=[{"role": "user", "content": "hello"}],
)
print(resp.choices[0].message.content)
```

### Tool Configuration

For any OpenAI-compatible tool (Cursor, Continue, aider, etc.):

- **Base URL**: `http://localhost:8787/v1`
- **API Key**: any value (or set `CODEX_LOCAL_AUTH` for authentication)
- **Model**: `gpt-5.4` (or any model from the list above)

## Supported Parameters

Parameters sent to the Codex API:

| Chat Completions Parameter | Codex Mapping | Notes |
|---|---|---|
| `messages[role=system]` | `instructions` | Required, defaults to "You are a helpful assistant." |
| `messages[role=user/assistant]` | `input` | Conversation messages |
| `model` | `model` | |
| `tools` | `tools` | |
| `tool_choice` | `tool_choice` | |
| `response_format` | `text` | |

Parameters silently ignored (not supported by Codex Responses API):

`temperature`, `top_p`, `max_tokens`, `max_completion_tokens`, `stop`, `frequency_penalty`, `presence_penalty`, `seed`, `user`

Automatically set by the proxy:

| Parameter | Value |
|---|---|
| `stream` | `true` (Codex requires streaming) |
| `store` | `false` |
| `reasoning.effort` | `"medium"` |
| `reasoning.summary` | `"auto"` |

## Configuration

| Environment Variable | Default | Description |
|---|---|---|
| `CODEX_LISTEN_ADDR` | `:8787` | Listen address |
| `CODEX_DATA_DIR` | `~/.codex-service` | Credentials storage directory |
| `CODEX_LOCAL_AUTH` | _(none)_ | Optional bearer token for incoming requests |

## How It Works

```
Client (Cursor, aider, etc.)
  │ POST /v1/chat/completions
  ▼
codex-service (localhost:8787)
  │ 1. Extract system messages → instructions
  │ 2. Transform Chat Completions → Responses format
  │ 3. Inject OAuth token + ChatGPT-Account-Id header
  ▼
chatgpt.com/backend-api/codex/responses
  │ SSE streaming response
  ▼
codex-service
  │ Transform Responses → Chat Completions format
  ▼
Client receives OpenAI-compatible response
```

## Disclaimer

This project is provided as-is for educational and personal use. It uses the same public OAuth endpoints as the official OpenAI Codex CLI, but is not affiliated with or endorsed by OpenAI. The author assumes no responsibility for any account suspension, data loss, or other consequences resulting from the use of this tool. Use at your own risk.

## License

MIT
