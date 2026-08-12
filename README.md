# Mini Notes with LLM Summary Anna App

A local Anna App that stores notes through the Anna App storage Host API and summarizes them through a bundled local Executa Tool. The summary path is intentionally:

```text
Anna App iframe
  -> AnnaAppRuntime.connect()
  -> anna.storage.get / anna.storage.set
  -> anna.tools.invoke(...)
  -> local Executa Tool invoke
  -> reverse JSON-RPC sampling/createMessage
  -> host LLM or local mock sampling fixture
  -> summary returned to UI
```

## Project structure

```text
.
├── manifest.json                         # Anna App schema 2 manifest
├── app.json                              # Local app metadata / bundled executa handle
├── src/                                  # React + Vite + TypeScript UI source
│   ├── services/                         # Anna runtime, storage, tools wrappers
│   └── App.tsx                           # Mini Notes UI
├── executas/mini-notes-summarizer/       # Go Executa Tool
│   ├── executa.json                      # Executa dev / distribution metadata
│   ├── cmd/mini-notes-summarizer/        # Binary entrypoint
│   └── internal/executa/                 # JSON-RPC + sampling implementation
├── fixtures/mock-sampling.jsonl          # anna-app executa dev sampling fixture
├── scripts/                              # Local and release packaging scripts
└── .github/workflows/release.yml         # GitHub Release asset workflow
```

## Prerequisites

- Node.js 20+
- npm 10+
- Go 1.21+
- Anna App CLI available as `anna-app`
- No Anna login, real LLM API key, or cloud database is required for local testing.

If your machine has a protected Go build cache, prefix Go/Executa commands with `GOCACHE=/private/tmp/anna-go-build-cache`.

## Install dependencies

```bash
npm install
```

## Build the frontend bundle

```bash
npm run build
```

Vite writes the static Anna App bundle to `bundle/`. `manifest.json` points `ui.bundle.entry` and the main view entry at `index.html` inside that bundle.

## Validate the Anna App manifest

```bash
anna-app validate --strict
```

This validates schema, static UI bundle files, `required_executas` references, and `ui.host_api` ACL coverage. If validation reports a local CLI/schema difference from staging docs, treat the CLI result as the source of truth.

## Run the UI harness with no LLM

```bash
npm run build
anna-app dev --no-llm
```

Open the dashboard URL printed by the CLI. Test:

1. Add a note.
2. Confirm it appears in the list with an order number.
3. Delete a note.
4. Click Summarize.

In `--no-llm` mode, clicking Summarize should still call `anna.tools.invoke(...)`. Because the harness disables LLM/sampling, the expected result is an error similar to:

```text
[-32603] harness started with --no-llm
```

That error is expected for the UI harness path and does not mean the Go Tool sampling implementation failed.

## Confirm notes use `anna.storage.*`

The UI storage code is isolated in `src/services/notesRepository.ts` and only calls:

- `anna.storage.get({ key: "mini-notes:v1:notes" })`
- `anna.storage.set({ key: "mini-notes:v1:notes", value: notes })`

The app does not use `localStorage`, IndexedDB, files, or React state as the persistence layer. React state only mirrors the latest storage-backed notes for rendering.

## Test Executa sampling with mock fixture

Use the Executa runner separately from the UI harness:

```bash
anna-app executa dev \
  --dir executas/mini-notes-summarizer \
  --mock-sampling fixtures/mock-sampling.jsonl \
  --invoke summarize \
  --args '{"notes":[{"order":1,"content":"Tomorrow follow up with customer"},{"order":2,"content":"Fix login bug"}],"max_words":80}'
```

The fixture in `fixtures/mock-sampling.jsonl` uses `ns: "sampling"`, `method: "createMessage"`, and `match.contentIncludes` to match the Tool's reverse sampling request, then returns a deterministic summary. To confirm sampling was actually initiated, inspect the runner output/logs for a reverse JSON-RPC request whose method is:

```text
sampling/createMessage
```

## Manually test Executa JSON-RPC

From the Tool directory:

```bash
cd executas/mini-notes-summarizer
```

Initialize with protocol v2:

```bash
echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2.0"}}' | go run ./cmd/mini-notes-summarizer
```

Describe the manifest:

```bash
echo '{"jsonrpc":"2.0","method":"describe","id":2}' | go run ./cmd/mini-notes-summarizer
```

Invoke without notes to verify validation and no fake summary:

```bash
echo '{"jsonrpc":"2.0","method":"invoke","id":3,"params":{"tool":"summarize","arguments":{"notes":[]}}}' | go run ./cmd/mini-notes-summarizer
```

For a successful summarize, prefer `anna-app executa dev --mock-sampling` because raw manual stdio also requires replying to the Tool's reverse `sampling/createMessage` request on the same stdin stream.

## Confirm summary uses `tools.invoke -> Executa -> sampling/createMessage`

- Frontend call site: `src/services/summarizeService.ts` uses `anna.tools.invoke` with `tool_id: "tool-dev-mini-notes-summarizer"` and `method: "summarize"`.
- Go invoke handler: `executas/mini-notes-summarizer/internal/executa/server.go` handles only the `summarize` tool.
- Sampling evidence: the Go Tool writes a reverse JSON-RPC request with method `sampling/createMessage` and metadata containing `executa_invoke_id`, `tool`, and `note_count`.
- Mock evidence: `anna-app executa dev --mock-sampling fixtures/mock-sampling.jsonl` returns the fixture summary only after that reverse request is observed by the runner.

## Run tests

Frontend tests:

```bash
npm run test
```

Go protocol/sampling tests:

```bash
go test ./executas/mini-notes-summarizer/...
```

All local unit tests:

```bash
npm run test:all
```

## Package the Executa binary locally

Build an archive for the current machine:

```bash
scripts/package-executa.sh
```

The script detects the local OS/architecture, compiles the Go binary, and writes an archive under `dist/executa/archives/`. Archive root contains:

```text
manifest.json
bin/mini-notes-summarizer      # or bin/mini-notes-summarizer.exe on Windows
```

Supported release platform keys are:

- `darwin-arm64`
- `darwin-x86_64`
- `windows-x86_64`

macOS assets use `.tar.gz`; Windows assets use `.zip`.

## GitHub Actions release

`.github/workflows/release.yml` supports manual `workflow_dispatch` and tags matching `mini-notes-summarizer-v*`. It builds and uploads these GitHub Release assets:

- `mini-notes-summarizer-darwin-arm64.tar.gz`
- `mini-notes-summarizer-darwin-x86_64.tar.gz`
- `mini-notes-summarizer-windows-x86_64.zip`

The workflow runs Go tests and a basic JSON-RPC `describe` smoke test before upload. Replace the placeholder `OWNER/REPO` URLs in `executas/mini-notes-summarizer/executa.json` with the real repository once the release location is known.

## Development document

A local copy of the requested Feishu development document is available at `anna-mini-notes-tech-design.md`. Creating the Feishu document requires a valid local `lark-cli` user authorization with `docx:document:create`.
