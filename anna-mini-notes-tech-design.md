# Mini Notes with LLM Summary Anna App 技术方案

## 1. 背景与目标

本项目要实现一个本地运行的 Anna App：Mini Notes with LLM Summary。用户可以在 Anna App UI 中创建、查看、删除笔记，并通过 Summarize 按钮调用本地 Executa Tool 生成总结。

本任务重点不是普通 Web App，而是验证 Anna 本地开发模型：UI bundle 必须运行在 Anna App Runtime 的 iframe 中，通过 Host API 调用 Anna storage 与 tools；总结必须由本地 Executa Tool 通过 reverse JSON-RPC `sampling/createMessage` 向 host LLM 或 mock sampling fixture 请求生成，不能由前端或 Tool 规则拼接。

## 2. 官方文档与示例仓库总结

已阅读的资料包括 Anna Developer Hub、Executa protocol/lifecycle/sampling/binary distribution 文档，以及 `anna-executa-examples` 示例仓库。

核心结论：

- Anna App 是面向终端用户的一键安装应用包，可声明所需 Executa、系统提示词与可选 UI bundle。`schema: 2` App 会在 dashboard 中以 sandbox iframe 加载静态 SPA。
- Executa 是 Anna 的插件扩展系统。Tool 是可执行进程，使用 JSON-RPC 2.0 over stdio；Skill 是声明式 Markdown recipe。本项目使用 Tool。
- App UI 通过 `AnnaAppRuntime.connect()` 建立与宿主的连接，所有 storage/tool 能力都走 Host API，并受 `manifest.ui.host_api` ACL 约束。
- Executa Tool 需要长期运行，持续逐行读取 stdin JSON-RPC request，逐行向 stdout 写 response；日志只能写 stderr，响应后必须 flush。
- Sampling 是 v2 reverse-RPC 能力。Tool 在 `initialize` 中声明 `client_capabilities.sampling = {}`，并在 `invoke` 内向 host 发起 `sampling/createMessage`，host response 会回到同一个 stdin reader。
- 本地开发路径分两条：UI harness 用 `anna-app dev --no-llm` 验证 iframe、storage、`tools.invoke` wiring；后端 sampling 用 `anna-app executa dev --mock-sampling <fixture.jsonl>` 单独验证。
- 发布路径需要 Executa binary distribution：按平台 key 提供 archive，macOS 使用 `.tar.gz`，Windows 使用 `.zip`，archive root 建议包含 `manifest.json` 指定 `runtime.binary.entrypoint`。

## 3. 五个必答问题

### 3.1 Anna App 是什么

Anna App 是 Anna 平台的应用包装层。开发者通过 manifest 声明应用元数据、所需 Executa、提示词、权限和可选 UI bundle。用户安装 App 后，可通过 App Store/mention/window 打开完整体验。对 UI App 而言，前端不是独立网站，而是由 Anna App Runtime 加载的静态 SPA。

### 3.2 Executa Tool 是什么

Executa Tool 是 Anna 可调用的本地插件进程。它不需要嵌入 Anna SDK，只要实现 JSON-RPC 2.0 over stdio 协议即可。Anna/Agent 会调用 `initialize`、`describe`、`invoke` 等方法，Tool 返回 manifest 和工具执行结果。

### 3.3 Anna App 如何通过 Host API 与本地 harness 交互

App UI bundle 在 iframe 中加载 SDK 后调用 `AnnaAppRuntime.connect()`。连接对象 `anna` 暴露 `anna.storage.get/set`、`anna.tools.invoke` 等 Host API。`anna-app dev` 本地 harness 使用与生产一致的 dispatcher，按 `manifest.ui.host_api` 校验 namespace/method 和 tool allow-list。无登录本地调试时 storage 使用 legacy in-memory runtime_state。

### 3.4 Executa Tool 如何通过 stdio 与 Anna 通信

Anna 向 Tool stdin 写入每行一个 JSON-RPC request。Tool 解析后向 stdout 写入对应 JSON-RPC response，一行一个 JSON，且 stdout 不允许混入日志。reverse RPC 时 Tool 也可以向 stdout 写 request，例如 `sampling/createMessage`；host 对该 request 的 response 会作为普通 JSON-RPC response 从 stdin 回来，Tool 需要按 id 匹配 pending request。

### 3.5 本地开发、测试和打包发布流程

本地开发先安装依赖并构建 UI bundle，再运行 `anna-app validate --strict` 验证 manifest、bundle 和 Host API ACL。UI 用 `anna-app dev --no-llm` 运行，验证 notes 的创建/查看/删除和 `tools.invoke` wiring；`--no-llm` 下 Summarize 返回 LLM disabled 错误是预期结果。后端用 `anna-app executa dev --mock-sampling fixtures/mock-sampling.jsonl` 验证 Tool 的 sampling 调用。发布时用 Go 交叉编译三平台二进制，按 Executa binary distribution 规范打包 archive，并通过 GitHub Actions 上传到 GitHub Release assets。

## 4. 技术栈选型

- 前端：React + Vite + TypeScript。原因是工程化清晰，容易拆分 UI、runtime、storage、tools service，并可用 Vitest 做单测。
- Tool：Go。原因是可直接产出三平台单二进制，符合“不接受源码 + 解释器运行作为发布产物”的要求。
- 测试：Vitest 覆盖前端业务逻辑；Go `testing` 覆盖协议、sampling、错误处理；shell smoke test 覆盖二进制 JSON-RPC。
- 打包：Go `GOOS/GOARCH` 交叉编译，macOS `.tar.gz`，Windows `.zip`，archive root 放 `manifest.json` 与 `bin/` entrypoint。
- CI：GitHub Actions matrix 构建三平台 release assets，执行 describe smoke test 后上传到 Release。

## 5. 实现设计

### 5.1 App manifest

`manifest.json` 使用 `schema: 2`，声明：

- `required_executas`: `bundled:mini-notes-summarizer`。
- `permissions`: `tools.invoke`、`storage.read`、`storage.write`。
- `ui.bundle.entry`: `index.html`，由 Vite 构建输出到 `bundle/`。
- `ui.views`: 单个 main view，标题 Mini Notes。
- `ui.host_api.storage`: `get`、`set`。
- `ui.host_api.tools`: `required:bundled:mini-notes-summarizer`。
- `dev`: 本地 fixtures、seed storage、user id。

### 5.2 前端模块

前端拆分为：

- runtime：封装 `AnnaAppRuntime.connect()`，提供测试 mock 注入点。
- notes repository：封装 `anna.storage.get/set`，storage key 固定为 `mini-notes:v1:notes`。
- summarize service：封装 `anna.tools.invoke({ tool_id, method: 'summarize', args, timeoutMs })`。
- UI：负责输入校验、列表展示、删除、错误状态和 summary 展示。

Note 数据结构：

```json
{
  "id": "string",
  "order": 1,
  "content": "string",
  "createdAt": "ISO-8601"
}
```

### 5.3 Go Executa Tool

Tool 支持：

- `initialize`: 协商 protocol v2，返回 `client_capabilities.sampling = {}`。
- `describe`: 返回裸 manifest，包含 `host_capabilities: ["llm.sample"]`、`tools[]` 和 `runtime`。
- `invoke`: 只处理 `summarize`。参数包含 `notes` array 与 `max_words`。
- `health`: 返回 ready/healthy。
- `shutdown`: 返回 ok，进程仍由 EOF 或 host 管理退出。

`summarize` 内部构建 prompt，调用 reverse RPC：

```json
{
  "jsonrpc": "2.0",
  "id": "sampling-...",
  "method": "sampling/createMessage",
  "params": {
    "messages": [{"role":"user","content":{"type":"text","text":"...notes..."}}],
    "maxTokens": 512,
    "systemPrompt": "You summarize short personal notes clearly and concisely.",
    "metadata": {"executa_invoke_id":"...", "tool":"summarize"}
  }
}
```

Tool 从 sampling response 的 `content.text` 取 summary，并以 `InvokeResult` 返回：

```json
{"success": true, "data": {"summary": "...", "model": "...", "usage": {...}}}
```

## 6. 测试案例

### 6.1 前端测试

- 空输入点击 Save 不调用 storage set。
- 保存有效输入后，notes 追加并写入 `anna.storage.set`，输入框清空。
- 初始化时调用 `anna.storage.get` 加载 notes。
- 删除一条 note 后立即更新列表并调用 storage set。
- Summarize 点击时调用 `anna.tools.invoke`，参数包含当前 notes 内容与 order。
- storage 或 tool 失败时展示错误状态。

### 6.2 Tool 测试

- `initialize` v2 返回 sampling capability。
- `describe` 返回包含 `host_capabilities: ["llm.sample"]` 与 `parameters[]` 的 manifest。
- 未知 method 返回 `-32601`。
- 未知 tool 返回 `-32601`。
- 空 notes 返回 `success: false` 或明确错误，不发起无意义 sampling。
- sampling 成功时 stdout 先出现 `sampling/createMessage` request，再在收到 mock response 后返回 invoke response。
- sampling error 时返回 JSON-RPC error 或 `success:false`，不伪造 summary。
- stdout 不包含日志、banner、debug 文本。

### 6.3 Harness 与发布验证

- `npm run build` 生成 `bundle/index.html` 和 assets。
- `anna-app validate --strict` 通过。
- `anna-app dev --no-llm` 中可创建、查看、删除 notes；Summarize 预期显示 `[-32603] harness started with --no-llm` 或等价错误。
- `anna-app executa dev --mock-sampling fixtures/mock-sampling.jsonl --invoke summarize --args ...` 返回 fixture summary，并能从 runner 输出确认 `sampling/createMessage` 被发起。
- 本地 `scripts/package-executa.sh` 生成当前平台 archive，并可对 archive 中二进制执行 `describe` smoke test。
- GitHub Actions 构建并上传三项 Release assets：`*-darwin-arm64.tar.gz`、`*-darwin-x86_64.tar.gz`、`*-windows-x86_64.zip`。

## 7. 验证链路

完整验证顺序：

1. `npm install` 安装前端依赖。
2. `npm run test` 运行前端测试。
3. `go test ./...` 运行 Tool 测试。
4. `npm run build` 构建 UI bundle。
5. `anna-app validate --strict` 验证 manifest 和 Host API ACL。
6. `anna-app dev --no-llm` 手动验证 UI + storage + tools.invoke wiring。
7. `anna-app executa dev --mock-sampling fixtures/mock-sampling.jsonl` 验证 Tool reverse sampling。
8. `scripts/package-executa.sh` 构建当前平台 binary archive。
9. GitHub Actions workflow 在 release/tag 或手动触发时构建三平台 assets 并上传 GitHub Release。

## 8. 风险与约束

- staging 文档可能与本地 CLI/schema 存在细微差异，最终以 `anna-app validate --strict` 为准。
- 无真实 Anna 登录与无真实 LLM key 是任务约束，因此 UI harness 不验证真实模型效果。
- `--no-llm` 下 Summarize 报错不是失败，而是证明调用链已进入 Tool/sampling 路径后的预期表现。
- Windows archive 不保留 Unix executable bit，因此 archive manifest 明确 entrypoint；macOS archive 中设置二进制可执行权限。
