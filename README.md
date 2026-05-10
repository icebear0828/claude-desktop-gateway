# Claude Desktop Gateway

中文 | [English](README.en.md)

Claude Desktop Gateway 是面向 Claude Desktop 第三方 provider 模式的本地网关和配置管理工具。它在本机暴露 Anthropic 兼容的 `/v1/models` 与 `/v1/messages` 接口，把 Claude Desktop 里的模型 ID 映射到真实上游模型，并优先转发到 OpenRouter。

这个项目不是通用 OpenRouter SDK，也不是托管代理。如果你在写自己的应用、脚本或服务，直接调用 OpenRouter 更简单。这个网关适合 Claude Desktop 场景：需要稳定的 Claude 风格模型 ID、本地可检查配置、免费模型 fallback、动态免费模型选择，以及不把密钥写进共享 JSON。

## 快速开始

本地 Claude Desktop 测试推荐使用脚本入口：

```bash
cp .env.local.example .env.local
cp gateway.local.example.json gateway.local.json
# 编辑 .env.local，填入 OPENROUTER_API_KEY 和 CLAUDE_GATEWAY_API_KEY。
./scripts/run-local
```

脚本会加载 `.env.local`，使用 `gateway.local.json`，并在 `http://127.0.0.1:8787` 启动网关。输出只包含变量名和文件路径，不打印密钥值。

只检查配置，不启动服务：

```bash
./scripts/run-local --dry-run
```

检查正在运行的网关：

```bash
curl http://127.0.0.1:8787/health
curl -H "Authorization: Bearer $CLAUDE_GATEWAY_API_KEY" http://127.0.0.1:8787/v1/models
```

正常给 Claude Desktop 使用时，建议后台运行：

```bash
scripts/local-gateway start
scripts/local-gateway status
scripts/local-gateway stop
```

修改配置后使用：

```bash
scripts/local-gateway restart
```

## Claude Desktop 配置

诊断当前实际生效的 Claude Desktop 3P profile：

```bash
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config doctor
```

应用或修复本地 Claude Desktop 3P profile：

```bash
source .env.local
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config apply-local
```

只预览不写入：

```bash
GOCACHE=/private/tmp/go-build-cache go run ./cmd/claude-desktop-config apply-local --dry-run
```

Claude Desktop profile 的核心字段形态：

```json
{
  "inferenceProvider": "gateway",
  "inferenceGatewayBaseUrl": "http://127.0.0.1:8787",
  "inferenceGatewayApiKey": "local-client-key",
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceModels": [
    "claude-ring-2-6-1t-free",
    "claude-free-auto",
    "claude-free-agent",
    "claude-free-coder",
    "claude-free-fast"
  ]
}
```

`apply-local` 会把 `CLAUDE_GATEWAY_API_KEY` 写入 Claude Desktop profile 供本地鉴权使用，输出会脱敏。设置 `CLAUDE_GATEWAY_CONFIG` 时，`inferenceModels` 会从网关路由推导。

## 模型路由

`gateway.local.json` 把 Claude Desktop 模型名和真实上游模型分开。默认 Ring 路由：

```json
{
  "providers": {
    "openrouter": {
      "profile": "anthropic-messages",
      "baseUrl": "https://openrouter.ai/api/v1",
      "apiKeyEnv": "OPENROUTER_API_KEY",
      "capabilities": {
        "streaming": true,
        "tools": true,
        "jsonMode": false
      }
    }
  },
  "routes": {
    "claude-ring-2-6-1t-free": [
      {
        "provider": "openrouter",
        "model": "inclusionai/ring-2.6-1t:free",
        "displayName": "OpenRouter Ring 2.6 1T Free",
        "cache": {
          "enabled": true,
          "ttlSeconds": 300
        }
      }
    ]
  }
}
```

路由 key 是 Claude Desktop 发送回 `/v1/messages` 的模型 ID；`model` 是发送到 provider 的真实上游模型。

默认别名：

```text
claude-ring-2-6-1t-free -> dedicated Ring 2.6 1T route
claude-free-auto        -> OpenRouter free router
claude-free-agent       -> 动态 tools-capable 免费模型，然后 fallback list
claude-free-coder       -> 动态 tools-capable 免费模型，然后 coding fallback list
claude-free-fast        -> 动态通用免费模型，然后 fast fallback list
```

`dynamicFreeModels` 会拉取 OpenRouter `/models` catalog，只保留零价格模型，并按 `requiredParameters`、上下文长度等条件过滤。`fallback` 只在 catalog 没有可用模型或动态路由遇到可重试上游错误时使用。

`cache.enabled` 会在上游请求中发送 OpenRouter response-cache headers。`anthropic-messages` profile 会保留 Anthropic 原生 `cache_control` block，因此 Claude Desktop 的 prompt-cache hint 不会被剥离。OpenRouter 返回 `X-OpenRouter-Cache-Status` 等 cache header 时，网关会转发给客户端。

OpenRouter 的 Anthropic Messages endpoint 使用：

```json
{ "profile": "anthropic-messages" }
```

此模式下，网关只重写模型 ID，保留 Anthropic 原生字段，例如 `tools`、`tool_choice`、`tool_result`、`cache_control` 和 `thinking`。只有 provider 不提供 Anthropic Messages endpoint 时，才使用 `profile: "openai-chat"`。

## 环境变量

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `OPENROUTER_API_KEY` | 必填 | OpenRouter 上游 API key |
| `CLAUDE_GATEWAY_API_KEY` | 无 | Claude Desktop 访问网关的客户端 key |
| `CLAUDE_GATEWAY_DEFAULT_MODEL` | `inclusionai/ring-2.6-1t:free` | 内置 alias 的默认目标模型 |
| `CLAUDE_MODEL_ALIASES` | 内置 alias | JSON object 或逗号分隔 alias map |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai/api/v1` | OpenRouter 兼容 base URL |
| `OPENROUTER_REFERRER` | 无 | 可选 OpenRouter attribution header |
| `OPENROUTER_TITLE` | `Codex Proxy Claude Gateway` | 可选 OpenRouter attribution header |
| `HOST` | `127.0.0.1` | 监听 host |
| `PORT` | `8787` | 监听端口 |
| `CLAUDE_GATEWAY_CONFIG` | 无 | 可选非密钥 JSON 配置文件路径，例如 `gateway.local.json` |

## LAN 或 VPS

让其他机器访问网关时，绑定到所有网卡：

```bash
cp gateway.lan.example.json gateway.lan.json
source .env.local
export CLAUDE_GATEWAY_CONFIG=gateway.lan.json
go run ./cmd/gateway
```

绑定到任何非 loopback host 时，Go core 要求设置 `CLAUDE_GATEWAY_API_KEY`，否则启动失败。Claude Desktop 的第三方 gateway 流程应使用 HTTPS，因此 LAN 模式建议启用 TLS。公网 VPS 不要直接暴露明文 HTTP；应放在 HTTPS 后面，配合防火墙，并使用强 `CLAUDE_GATEWAY_API_KEY`。

## Desktop GUI

GUI 是 Wails 桌面应用，读取现有 Go/config 状态，不重复实现网关协议逻辑。目前展示 gateway health、listen URL、配置错误、providers、route aliases 和 Claude Desktop doctor 状态。

直接启动：

```bash
GOCACHE=/private/tmp/go-build-cache go run .
```

构建本地 macOS app：

```bash
GOCACHE=/private/tmp/go-build-cache /Users/c/go/bin/wails build
```

发布版 GUI 找不到源码仓库时，会使用系统用户配置目录并自动创建默认 `gateway.local.json`，配置文件里只引用 env var，不写入密钥。

## 跨平台发布

桌面 prerelease 通过 Git tag 发布，例如 `v0.1.0-pre.1`。GitHub Actions 会在原生 runner 上构建 unsigned Wails GUI，并上传：

- `claude-desktop-gateway-<tag>-macos-arm64.zip`
- `claude-desktop-gateway-<tag>-macos-amd64.zip`
- `claude-desktop-gateway-<tag>-windows-amd64.zip`
- `claude-desktop-gateway-<tag>-linux-amd64.tar.gz`

这些是未签名、未公证的预览构建：

- macOS 首次打开可能需要在 Privacy & Security 里手动允许。
- Windows 可能出现 SmartScreen 提示。
- Linux 需要 GTK 3 和 WebKitGTK 4.1 runtime libraries。

## TypeScript 参考实现

TypeScript 服务保留为 Go 重写期间的行为参考：

```bash
export OPENROUTER_API_KEY="..."
export CLAUDE_GATEWAY_API_KEY="local-client-key"
npm run dev
```

默认地址：`http://127.0.0.1:8787`

## 测试

本地测试：

```bash
GOCACHE=/private/tmp/go-build-cache go test ./...
npm test
npm run build
```

真实 OpenRouter 三连测试需要先导出 `OPENROUTER_API_KEY`：

```bash
GOCACHE=/private/tmp/go-build-cache go test ./internal/gateway -run TestRealOpenRouterCompletesThreeSequentialCalls -count=1
GOCACHE=/private/tmp/go-build-cache go test ./internal/gateway -run TestRealOpenRouterAnthropicMessagesCompletesThreeSequentialCalls -count=1
GOCACHE=/private/tmp/go-build-cache go test ./internal/gateway -run 'TestRealOpenRouterAnthropicMessages(Streams|Tools)ThreeSequentialCalls' -count=1
```

每个真实测试都会对 `inclusionai/ring-2.6-1t:free` 连续调用 3 次。免费上游 provider 可能返回 429；在标记 Claude Desktop 兼容性完成前，应使用 route fallback 或更稳定的上游模型复测。
