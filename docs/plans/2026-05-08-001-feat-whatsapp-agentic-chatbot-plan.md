---
title: "feat: WhatsApp Agentic AI Chatbot"
type: feat
status: active
date: 2026-05-08
origin: docs/brainstorms/whatsapp-agentic-chatbot-requirements.md
---

# feat: WhatsApp Agentic AI Chatbot

## Summary

Implementasi chatbot WhatsApp berbasis Go + whatsmeow dengan sistem agentic AI (tool calling via OpenAI-compatible API), hybrid tool system (YAML + Go interface), session memory per nomor via SQLite, dan allowlist keamanan berbasis config TOML.

---

## Problem Frame

Saat ini belum ada implementasi — greenfield project. Sistem yang akan dibangun memungkinkan pengguna berinteraksi dengan AI agent melalui WhatsApp, di mana agent dapat mengeksekusi beragam tools (web search, HTTP request, kalkulator, dll) secara otomatis berdasarkan kebutuhan percakapan (see origin: docs/brainstorms/whatsapp-agentic-chatbot-requirements.md).

---

## Requirements

**Conversation & Agent**
- **R1.** Bot menerima dan mengirim pesan WhatsApp (teks) menggunakan library whatsmeow.
- **R2.** Bot memanggil LLM via OpenAI-compatible API endpoint (`/chat/completions`) dengan konfigurasi base URL, API key, model, dan temperature dari config file TOML.
- **R3.** Agent mendeteksi kebutuhan tool calling dari respons LLM, mengeksekusi tool yang diminta secara synchronous, dan mengembalikan hasil eksekusi ke LLM untuk menghasilkan respons final.
- **R4.** Jika LLM tidak memanggil tool, bot merespons langsung dalam satu pesan WhatsApp.

**Tool System**
- **R5.** Tool sederhana didefinisikan sebagai file YAML yang berisi: nama, deskripsi, parameter (nama, tipe, required), dan command (shell command atau HTTP request template).
- **R6.** Tool kompleks didefinisikan sebagai file Go yang mengimplementasikan interface `Tool` (menyediakan metadata tool dan fungsi `Execute`).
- **R7.** Bot memindai dan memuat semua tool dari folder yang ditentukan saat startup (auto-load).
- **R8.** Bot menyertakan sekumpulan built-in tools dasar: web search, HTTP request, kalkulator, dan datetime.

**Security**
- **R9.** Bot hanya merespons nomor WhatsApp yang terdaftar dalam allowlist di config file TOML. Pesan dari nomor di luar allowlist diabaikan.
- **R10.** Seluruh konfigurasi bot — allowlist, kredensial LLM, pengaturan tool, session — disimpan dalam satu config file TOML.

**Session & Memory**
- **R11.** Session memory per nomor WhatsApp disimpan di SQLite. Setiap nomor memiliki tepat satu session.
- **R12.** Session menyimpan conversation history (urutan pesan user dan assistant) yang digunakan sebagai context saat memanggil LLM.

**Origin actors:** A1 (End User), A2 (Admin), A3 (Bot Agent)
**Origin flows:** F1 (Percakapan tanpa tool), F2 (Percakapan dengan tool calling), F3 (Menambah tool baru)
**Origin acceptance examples:** AE1 (covers R1, R2, R4), AE2 (covers R1, R2, R3), AE3 (covers R9), AE4 (covers R5, R7), AE5 (covers R11, R12)

---

## Scope Boundaries

- Tidak menyediakan web dashboard atau admin panel — semua konfigurasi via file TOML.
- Tidak mendukung multi-platform — WhatsApp hanya via whatsmeow.
- Tidak ada rate limiting atau usage tracking.
- Tidak ada long-term memory berbasis embedding/vektor.
- Tidak mendukung group chat WhatsApp.
- Tidak mendukung multi-tenant atau multi-bot dalam satu instance.
- Tidak ada proaktif push notifikasi — bot hanya merespons pesan masuk.

---

## Context & Research

### Relevant Code and Patterns

- **whatsmeow** (`go.mau.fi/whatsmeow`): Library WhatsApp Multi-Device API. Client dibuat via `whatsmeow.NewClient(deviceStore, nil)`, event handler via `client.AddEventHandler()`, message diterima sebagai `*events.Message`, pairing via QR channel.
- **whatsmeow** (`go.mau.fi/whatsmeow`): Library WhatsApp Multi-Device API. Client dibuat via `whatsmeow.NewClient(deviceStore, nil)`, event handler via `client.AddEventHandler()`, message diterima sebagai `*events.Message`, pairing via QR channel.
- **go-sqlite3** (`github.com/mattn/go-sqlite3`): Driver SQLite standar, juga digunakan oleh whatsmeow untuk device store.
- **TOML** (`github.com/BurntSushi/toml`): Library TOML parsing standar Go.
- **YAML** (`gopkg.in/yaml.v3`): Library YAML parsing untuk tool definitions.
- **LLM client**: Custom `net/http` — tidak pakai library go-openai. Langsung JSON marshal/unmarshal untuk OpenAI-compatible endpoint.

### Key API Patterns (dari research)

**whatsmeow — Inisialisasi & Pairing:**
```go
container, _ := sqlstore.New("sqlite3", "file:whatsmeow.db?_foreign_keys=on", nil)
deviceStore, _ := container.GetFirstDevice()
client := whatsmeow.NewClient(deviceStore, nil)
client.AddEventHandler(func(evt interface{}) {
    switch v := evt.(type) {
    case *events.Message:
        sender := v.Info.Sender.User  // nomor pengirim
        text := v.Message.GetConversation()
    }
})
// QR pairing jika deviceStore.ID == nil
qrChan, _ := client.GetQRChannel(context.Background())
client.Connect()
```

**Custom HTTP Client — Tool Calling Pattern (directional, bukan library yang dipakai):**
```go
// Struct internal (bukan dari library go-openai)
type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Tools    []ToolDef `json:"tools,omitempty"`
}
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}
type ToolCall struct {
    ID       string       `json:"id"`
    Function FunctionCall `json:"function"`
}
// Kirim via http.Post dengan JSON, parse response, loop jika ada tool_calls

### External References

- [whatsmeow Getting Started](https://deepwiki.com/tulir/whatsmeow/1.2-getting-started)
- [whatsmeow Messaging](https://deepwiki.com/tulir/whatsmeow/3-messaging)
- [go-openai tool calling example](https://gist.github.com/marcboeker/6177ea03201c94ac188ef8c3dede970a)
- [go-openai with custom base URL](https://deepwiki.com/sashabaranov/go-openai)

---

## Key Technical Decisions

- **Dua database SQLite terpisah**: whatsmeow butuh SQLite untuk device store sendiri (`whatsmeow.db`). Session memory bot disimpan di SQLite terpisah (`session.db`) untuk isolasi — whatsmeow mengelola schema-nya sendiri secara internal.
- **Custom HTTP client untuk LLM (tanpa go-openai)**: Karena endpoint-nya local gateway (`http://localhost:20128/v1`) yang mungkin tidak sepenuhnya kompatibel dengan go-openai, kita gunakan `net/http` langsung dengan struct JSON. Ini lebih ringan dan tanpa dependensi eksternal. Struct: `ChatCompletionRequest`, `ChatCompletionResponse`, `Tool`, `ToolCall`, `Message`.
- **Tool interface tunggal**: Semua tool (baik dari YAML maupun Go) menghasilkan `openai.Tool` + fungsi `Execute(argsJSON string) (string, error)` yang seragam. YAML tool di-wrap jadi implementasi interface yang sama.
- **YAML tool mendukung dua executor**: `shell` (menjalankan command via `os/exec`) dan `http` (melakukan HTTP request dengan template URL/body). Cukup untuk hampir semua tool simpel.
- **Allowlist sebagai map di memory**: Allowlist dari TOML di-load ke `map[string]bool` saat startup. Lookup O(1) untuk setiap pesan masuk.
- **Session per nomor, bukan per chat**: Satu nomor = satu session history. Kalau nomor yang sama chat dari konteks berbeda, history tetap digabung (sesuai R11).
- **Shell execution via `exec.CommandContext` dengan direct argv**: Shell command dijalankan sebagai `exec.CommandContext(ctx, binary, arg1, arg2)` — bukan `sh -c`. Ini mencegah command injection karena parameter LLM menjadi argv elements, bukan bagian dari shell string.
- **SSRF protection untuk HTTP executor**: HTTP tool mem-blokir request ke IP private (RFC 1918), loopback, link-local, dan local LLM gateway. Redirect limit 5, response body limit 1MB.
- **API key via environment variable**: `LLM_API_KEY` env var adalah primary source. Config TOML hanya fallback (dan sebaiknya kosong). `config.toml` di-gitignore, `config.example.toml` dengan placeholder aman di-commit.
- **JID normalization**: WhatsApp JID (`628xxx@s.whatsapp.net`) dinormalisasi ke nomor telepon (`628xxx`) via fungsi `extractPhone()` sebelum dicocokkan dengan allowlist.

---

## Output Structure

```
maingo/
├── main.go                  # entry point: load config, init semua, start bot
├── go.mod
├── go.sum
├── config.toml              # konfigurasi utama
├── system-prompt.txt        # system prompt terpisah
├── tools/
│   ├── definitions/         # YAML tool files (*.yml)
│   │   ├── web_search.yml
│   │   ├── http_request.yml
│   │   ├── calculator.yml
│   │   └── datetime.yml
│   └── custom/              # Go tool files (*.go, di-compile bersama binary)
├── internal/
│   ├── config/
│   │   └── config.go        # TOML parsing, struct Config
│   ├── whatsapp/
│   │   └── client.go        # whatsmeow init, pairing, event handler, send message
│   ├── llm/
│   │   └── client.go        # HTTP client ke OpenAI-compatible API, tool calling loop
│   ├── tool/
│   │   ├── registry.go      # Tool interface, registry, auto-load YAML + Go tools
│   │   └── yaml_tool.go     # YAML tool parser & executor (shell + http)
│   ├── session/
│   │   └── store.go         # SQLite session store: init, load, save, per-number
│   └── agent/
│       └── loop.go          # Agent loop: terima pesan → allowlist → session → LLM → reply
```

---

## Implementation Units

### U1. Project Scaffolding & Config

**Goal:** Inisialisasi project Go, struktur folder, modul, config TOML parsing, dan system prompt file.

**Requirements:** R2, R10

**Dependencies:** None

**Files:**
- Create: `go.mod`
- Create: `main.go` (skeleton — init config, start bot, graceful shutdown)
- Create: `config.toml`
- Create: `config.example.toml` (template dengan placeholder, aman di-commit)
- Create: `.gitignore` (abaikan `config.toml`, `*.db`, `whatsmeow.db`)
- Create: `system-prompt.txt`
- Create: `internal/config/config.go`

**Approach:**
- `go mod init maingo` — nama module lokal
- Config TOML mencakup: `[whatsapp]` (allowlist = array nomor), `[llm]` (base_url, api_key = "" atau env override, model, temperature, max_tool_rounds), `[tools]` (definitions_dir, custom_dir, shell_timeout_sec, http_timeout_sec), `[session]` (db_path, max_history)
- API key: prioritaskan env var `LLM_API_KEY` — fallback ke `api_key` di TOML jika env kosong. Jangan simpan API key asli di `config.toml` yang di-commit.
- Validasi file permission `config.toml` saat startup: warning jika bukan `0600`
- `config.example.toml` berisi semua field dengan nilai placeholder — aman di-commit ke git
- `.gitignore`: `config.toml`, `*.db`, `session.db`
- Struct `Config` di-parse via `github.com/BurntSushi/toml`
- System prompt dibaca dari file terpisah (`system-prompt.txt`) — path dikonfigurasi di TOML

**Contoh `config.toml`:**
```toml
[whatsapp]
allowlist = ["6281234567890", "6289876543210"]

[llm]
base_url = "http://localhost:20128/v1"
api_key = ""  # kosongkan, isi via env LLM_API_KEY
model = "oc/minimax-m2.5-free"
temperature = 0.7
max_tool_rounds = 5

[tools]
definitions_dir = "tools/definitions"
custom_dir = "tools/custom"
shell_timeout_sec = 30
http_timeout_sec = 15

[session]
db_path = "session.db"
max_history = 50
```

**Patterns to follow:**
- Standard Go project layout — `internal/` untuk package private
- TOML struct tags: `toml:"field_name"`

**Test scenarios:**
- Happy path: `config.toml` valid → semua field ter-parse dengan benar
- Happy path: `system-prompt.txt` ada → terbaca sebagai string
- Edge case: Config file tidak ditemukan → error jelas saat startup
- Edge case: Field wajib kosong (api_key, base_url) → error validasi

**Verification:**
- `go build` sukses
- Print config saat startup (tanpa API key) mengonfirmasi semua nilai terbaca

---

### U2. WhatsApp Client

**Goal:** Inisialisasi whatsmeow client, pairing QR code / auto-login, event handler untuk menerima dan mengirim pesan, dan pengecekan allowlist.

**Requirements:** R1, R9

**Dependencies:** U1 (config)

**Files:**
- Create: `internal/whatsapp/client.go`
- Modify: `main.go` (integrate)

**Approach:**
- Gunakan `go.mau.fi/whatsmeow` + `go.mau.fi/whatsmeow/store/sqlstore`
- Device store via SQLite (`whatsmeow.db`), terpisah dari session store
- Cek `deviceStore.ID == nil` → pairing flow dengan QR channel; jika sudah ada → langsung `Connect()`
- Event handler `AddEventHandler` menangani `*events.Message`, `*events.Connected`, `*events.Paired`, `*events.LoggedOut`
- **JID normalization**: WhatsApp JID format `628xxx@s.whatsapp.net`. Fungsi `extractPhone(jid string) string` split di `@` dan ambil local part. Allowlist di-load sebagai `map[string]bool` dengan key nomor yang sudah dinormalisasi.
- **Non-text filter**: Guard di awal event handler — cek `msg := v.Message; conversation := msg.GetConversation()`; jika `conversation == ""`, abaikan pesan (gambar, stiker, video, dll tidak diproses)
- Fungsi `SendReply(ctx, recipientJID types.JID, text string)` untuk kirim balasan

**Patterns to follow:**
- whatsmeow pattern dari [DeepWiki](https://deepwiki.com/tulir/whatsmeow/1.2-getting-started): `sqlstore.New` → `GetFirstDevice` → `NewClient` → `AddEventHandler` → `Connect`
- `client.EnableAutoReconnect = true` untuk ketahanan koneksi

**Test scenarios:**
- Happy path: Pesan masuk dari nomor ter-allowlist → message text diterima oleh handler
- Happy path: `extractPhone("6281234567890@s.whatsapp.net")` → `"6281234567890"`
- Happy path: `SendReply` → pesan terkirim ke nomor tujuan
- Edge case: Pesan dari nomor tidak ter-allowlist → diabaikan (tidak ada reply)
- Edge case: `deviceStore.ID == nil` → QR channel aktif, pairing berhasil setelah scan
- Edge case: Pesan non-teks (gambar, stiker) → `GetConversation()` kosong, pesan diabaikan

**Verification:**
- Bot bisa pairing dengan WhatsApp (QR muncul di terminal)
- Setelah paired, connect ulang tanpa QR (auto-login)
- Pesan dari nomor allowlist diproses, dari luar diabaikan

---

### U3. Session Store

**Goal:** Penyimpanan dan pengambilan conversation history per nomor WhatsApp via SQLite.

**Requirements:** R11, R12

**Dependencies:** U1 (config — db path)

**Files:**
- Create: `internal/session/store.go`

**Approach:**
- Database SQLite terpisah (`session.db`) dari whatsmeow device store
- Schema: tabel `sessions` (phone_number TEXT PRIMARY KEY, messages JSON TEXT, updated_at INTEGER)
- Messages disimpan sebagai JSON array of `{role, content, tool_call_id?, tool_calls?}`
- Fungsi: `Load(phone string) ([]Message, error)`, `Save(phone string, messages []Message) error`
- Limit: maksimal N message terakhir (default 50) untuk kontrol context window
- Gunakan `database/sql` + `github.com/mattn/go-sqlite3`

**Patterns to follow:**
- SQLite dengan `_foreign_keys=on` seperti whatsmeow
- JSON marshaling via `encoding/json`

**Test scenarios:**
- Happy path: Simpan messages untuk nomor baru → tersimpan, load mengembalikan data yang sama
- Happy path: Update session nomor yang sudah ada → messages terbaru tersimpan
- Edge case: Load nomor yang belum ada session → return empty slice
- Edge case: Messages melebihi limit → hanya N terbaru yang disimpan

**Verification:**
- `go test ./internal/session/` — test dengan SQLite in-memory
- Session bertahan setelah restart (cek file `session.db`)

---

### U4. Tool System

**Goal:** Definisikan `Tool` interface, YAML tool parser + executor, Go tool registry, auto-load dari folder, dan built-in tools.

**Requirements:** R5, R6, R7, R8

**Dependencies:** U1 (config — tools dir paths)

**Files:**
- Create: `internal/tool/registry.go`
- Create: `internal/tool/yaml_tool.go`
- Create: `tools/definitions/web_search.yml`
- Create: `tools/definitions/http_request.yml`
- Create: `tools/definitions/calculator.yml`
- Create: `tools/definitions/datetime.yml`

**Approach:**
- Interface `Tool`: `Metadata() ToolDef` (nama, deskripsi, parameters jsonschema) + `Execute(argsJSON string) (string, error)`
- `ToolDef` digunakan untuk generate `openai.Tool` / JSON schema tool definition
- **YAML tools** (`tools/definitions/*.yml`): format YAML dengan `name`, `description`, `parameters` (list of `{name, type, required, description, validation_regex?}`), dan `executor` (`shell` dengan `command` template, atau `http` dengan `url`, `method`, `headers`, `body` template). Template menggunakan `{{.param_name}}` untuk substitusi parameter.
- **Shell executor safety**: Gunakan `exec.CommandContext(ctx, binary, arg1, arg2, ...)` dengan argumen langsung — JANGAN pakai `sh -c`. Parameter LLM menjadi argv elements, bukan shell string yang diinterpolasi. Jika tool spesifik benar-benar butuh shell, gunakan executor type `raw_shell` terpisah dengan opt-in eksplisit dan dokumentasi risiko.
- **Parameter validation**: Sebelum eksekusi, validasi setiap parameter LLM terhadap `type` yang dideklarasikan di YAML (`string`, `number`, `boolean`). Tolak parameter yang tidak match. Jika `validation_regex` disetel, parameter string harus lolos regex.
- **HTTP executor SSRF protection**: Blokir IP private/localhost (RFC 1918, 127.0.0.0/8, 169.254.0.0/16) dan local LLM gateway endpoint. Redirect limit maks 5. Response body limit 1MB via `io.LimitReader`.
- **Tool execution timeout**: Semua tool menerima `context.Context`; shell via `exec.CommandContext`, HTTP via `http.NewRequestWithContext`. Timeout dari config (`shell_timeout_sec=30`, `http_timeout_sec=15`). Timeout → return error jelas ke LLM.
- **Go tools** (`tools/custom/`): file `.go` yang implement `Tool` interface dan di-register via `init()`
- `Registry.Scan()` membaca folder definitions dan mendaftarkan semua YAML + Go tools
- Built-in tools (R8) sebagai file YAML di `tools/definitions/`, bukan hard-coded di Go

**Patterns to follow:**
- Interface-based polymorphism untuk uniform tool execution
- `text/template` untuk substitusi parameter di YAML executor
- `os/exec` untuk shell executor, `net/http` untuk HTTP executor
- YAML parsing via `gopkg.in/yaml.v3`

**Test scenarios:**
- Happy path: YAML tool dengan shell executor → parameter disubstitusi, command dijalankan via `exec.CommandContext`, output dikembalikan
- Happy path: YAML tool dengan HTTP executor → URL template diisi, request dijalankan, response body dikembalikan (max 1MB)
- Happy path: `Registry.Scan()` → semua YAML dari folder ter-load
- Edge case: YAML tool dengan parameter tidak lengkap → error deskriptif
- Edge case: Folder definitions kosong → registry kosong tanpa error
- Edge case: Parameter tidak match tipe yang dideklarasikan → ditolak
- Edge case: Parameter tidak lolos `validation_regex` → ditolak
- Error path: Shell command gagal → error dikembalikan, tidak crash
- Error path: Shell command timeout → `context.DeadlineExceeded`, return error timeout
- Error path: HTTP request ke IP private (127.0.0.1, 10.x, 192.168.x) → ditolak sebelum request
- Error path: HTTP request timeout → return error timeout

**Verification:**
- `go test ./internal/tool/` — test registry load dan executor
- Folder `tools/definitions/` berisi 4 built-in YAML tools yang valid

---

### U5. LLM Client

**Goal:** HTTP client ke OpenAI-compatible API endpoint, membangun conversation dengan system prompt + context + tools, dan tool calling loop.

**Requirements:** R2, R3, R4

**Dependencies:** U3 (session — load/save history), U4 (tool registry — untuk dapatkan tool definitions)

**Files:**
- Create: `internal/llm/client.go`

**Approach:**
- Client struct menyimpan baseURL, apiKey, model, temperature, httpClient (dengan Timeout 60s)
- Fungsi utama: `Chat(ctx, messages []Message, tools []ToolDef) (*Response, error)`
- Request body: JSON `{model, messages, tools, temperature, stream: false}`
- Response parsing: cek `choices[0].message.tool_calls` — jika ada, return tool calls; jika tidak, return content
- Response body dibaca dengan `io.LimitReader` (max 512KB) untuk cegah response raksasa
- Loop di-handle di agent layer (U6), bukan di LLM client — jadi client hanya single call
- Message struct internal: `{Role, Content, ToolCallID, ToolCalls}` — seragam dengan session store

**Patterns to follow:**
- Standard `net/http` dengan JSON marshal/unmarshal — tanpa dependensi eksternal
- Context propagation untuk cancellation/timeout
- Custom `BaseURL` support — set via config

**Test scenarios:**
- Happy path: Request tanpa tools → response content dikembalikan
- Happy path: Request dengan tools → response berisi tool_calls
- Error path: API unreachable → error dengan context
- Error path: Response non-200 → error dengan status code
- Edge case: Response kosong (no choices) → error

**Verification:**
- Manual test: panggil endpoint local dengan curl-like test
- Unit test dengan `httptest.Server` mock

---

### U6. Agent Loop

**Goal:** Menggabungkan semua komponen: menerima pesan WhatsApp → cek allowlist → load session → kirim ke LLM dengan tools → handle tool calls → simpan session → kirim balasan.

**Requirements:** R1, R2, R3, R4, R9, R11, R12

**Dependencies:** U2 (WhatsApp client), U3 (Session store), U4 (Tool registry), U5 (LLM client)

**Files:**
- Create: `internal/agent/loop.go`
- Modify: `main.go` (wire semua)

**Approach:**
- `Agent` struct memegang reference ke WhatsApp client, LLM client, session store, tool registry, allowlist, system prompt
- Flow: `HandleMessage(ctx, senderPhone, messageText)`:
  1. Cek allowlist → return jika tidak diizinkan
  2. Validasi panjang `messageText` (max 4000 karakter) — tolak dengan balasan jika terlalu panjang
  3. Load session history untuk nomor tersebut
  4. Bangun messages array: system prompt + history + user message baru
  5. Load tool definitions dari registry
  6. Panggil LLM — jika response ada `tool_calls`, eksekusi tool via registry dengan timeout, tambahkan hasil sebagai `role: tool`, panggil LLM lagi (loop sampai tidak ada tool_calls atau max `max_tool_rounds` = 5 tercapai)
  7. Tool result di-truncate ke 64KB sebelum dikirim kembali ke LLM
  8. Simpan semua messages baru ke session store
  9. Kirim response text akhir ke WhatsApp (truncate ke 4096 karakter — batas WhatsApp)
- WhatsApp event handler di `main.go` memanggil `agent.HandleMessage` untuk setiap `*events.Message`

**Graceful shutdown** (di `main.go`):
- Trap `SIGINT`/`SIGTERM` via `signal.NotifyContext`
- Propagate context cancellation ke agent loop, HTTP client, dan tool execution
- Panggil `client.Disconnect()` pada whatsmeow client
- Pastikan semua session tersimpan sebelum exit

**Patterns to follow:**
- Tool calling loop pattern dari [go-openai gist](https://gist.github.com/marcboeker/6177ea03201c94ac188ef8c3dede970a): loop while `len(toolCalls) > 0`, eksekusi semua tool calls, tambahkan sebagai tool messages, kirim ulang
- Semua operasi synchronous dalam satu goroutine per pesan

**Test scenarios:**
- Happy path: Pesan tanpa butuh tool → 1 LLM call, response langsung (Covers F1, AE1)
- Happy path: Pesan butuh tool → LLM call dengan tool_calls → eksekusi → LLM call kedua → response final (Covers F2, AE2)
- Happy path: Session bertahan → "Nama saya Budi" → "Siapa nama saya?" → ingat (Covers AE5)
- Edge case: Nomor tidak di-allowlist → tidak ada respons (Covers AE3)
- Edge case: Pesan >4000 karakter → ditolak dengan balasan "pesan terlalu panjang"
- Edge case: Tool execution gagal → error dikembalikan ke LLM, LLM memberi respons yang sesuai
- Edge case: Session kosong (first message) → hanya system prompt + pesan baru
- Edge case: Tool calling mencapai max_rounds (5) → stop loop, kirim respons terakhir
- Error path: LLM call timeout → error handling, balasan "coba lagi nanti"
- Error path: LLM return >1 tool_calls → semua dieksekusi, hasil dikumpulkan
- Graceful shutdown: SIGTERM → context cancel → disconnect → session flush → exit

**Verification:**
- End-to-end: kirim pesan WhatsApp → terima balasan
- End-to-end: kirim pesan yang butuh web search → dapat hasil pencarian
- Session: dua pesan berurutan → bot ingat konteks

---

## System-Wide Impact

- **Interaction graph:** `main.go` → WhatsApp event handler → Agent.HandleMessage → Session.Load → LLM.Chat → Tool.Execute → Session.Save → WhatsApp.SendReply
- **Error propagation:** Setiap layer mengembalikan error; agent loop menangkap error dan mengirim pesan error yang ramah ke user ("Maaf, ada kendala teknis. Coba lagi nanti.") tanpa membocorkan detail internal
- **State lifecycle risks:** Dua database SQLite (`whatsmeow.db` + `session.db`) — pastikan path tidak bentrok. whatsmeow mengelola schema-nya sendiri, session schema dikelola manual via migrasi sederhana. Session di-flush saat graceful shutdown.
- **Shutdown sequence:** SIGTERM → `signal.NotifyContext` cancel → agent loop selesai → `client.Disconnect()` → session store flush → exit. Context cancellation propagasi ke semua goroutine (LLM calls, tool execution).
- **Unchanged invariants:** whatsmeow device store tidak disentuh oleh kode kita — hanya digunakan via API publik (GetFirstDevice, NewClient)

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| whatsmeow library berubah API (breaking change) | Gunakan `go.mod` untuk pin versi spesifik. whatsmeow cukup stabil di `go.mau.fi/whatsmeow` |
| Local LLM gateway tidak support tool calling penuh | Uji dengan request minimal dulu; fallback ke no-tool conversation |
| QR pairing timeout / gagal | Tampilkan instruksi jelas di terminal, support regenerate QR |
| SQLite concurrent access (multiple goroutine) | Gunakan `sync.Mutex` di session store; whatsmeow handle sendiri |
| Shell executor tools — command injection via LLM-provided parameters | Gunakan `exec.CommandContext` dengan direct argv (bukan `sh -c`); validasi parameter terhadap tipe yang dideklarasikan; `validation_regex` opsional per parameter; `raw_shell` executor terpisah dengan opt-in eksplisit |
| SSRF via built-in http_request tool | Blokir IP private/loopback/link-local + localhost gateway; redirect limit 5; port allowlist (default 80, 443) |
| API key terekspos via config file tercommit | `.gitignore` untuk `config.toml`; env var `LLM_API_KEY` sebagai primary source; `config.example.toml` untuk template; validasi permission `0600` |
| Allowlist mismatch JID vs phone number format | Fungsi `extractPhone()` — split `@`, ambil local part; validasi format saat startup; unit test dengan JID asli whatsmeow |

---

## Open Questions

### Resolved During Planning

- **Struktur folder proyek**: Standard Go layout — `internal/` untuk packages, `tools/` untuk tool files, config di root
- **Definisi interface Tool**: Satu interface dengan `Metadata()` + `Execute(argsJSON)` — YAML dan Go tools implement interface yang sama
- **Format YAML tool**: `name`, `description`, `parameters[]`, `executor` (shell/http) dengan Go template `{{.param}}`
- **Mekanisme auto-load**: Filesystem scan di `Registry.Scan()` — baca folder, parse `.yml`, daftarkan
- **Struktur tabel SQLite**: Tabel `sessions` dengan `phone_number TEXT PK`, `messages JSON`, `updated_at INTEGER`
- **Custom HTTP client vs go-openai**: `net/http` langsung — lebih ringan, tidak bergantung kompatibilitas library dengan local gateway

### Deferred to Implementation

- **[Mempengaruhi R1]** [Needs research] Cara pairing WhatsApp Multi-Device — QR code di terminal vs pairing code — tergantung preferensi deploy
- **[Mempengaruhi R5]** [Needs research] Implementasi konkrit `validation_regex` per parameter YAML tool — pattern regex dan error message yang sesuai
- **[Mempengaruhi R5]** [Needs research] Apakah butuh `raw_shell` executor untuk tool yang benar-benar memerlukan shell interpreter? Default: tidak ada, tambah hanya jika diperlukan
- **[Mempengaruhi R12]** Jumlah maksimal message history yang disimpan — ditentukan saat testing dengan real usage
- **[Mempengaruhi R5]** [Needs research] Web search tool implementation — API key search engine (Google/Bing/etc) atau cukup scraping?

---

## Sources & References

- **Origin document:** [docs/brainstorms/whatsapp-agentic-chatbot-requirements.md](docs/brainstorms/whatsapp-agentic-chatbot-requirements.md)
- [whatsmeow Getting Started — DeepWiki](https://deepwiki.com/tulir/whatsmeow/1.2-getting-started)
- [whatsmeow Messaging — DeepWiki](https://deepwiki.com/tulir/whatsmeow/3-messaging)
- [OpenAI tool calling in Go — gist](https://gist.github.com/marcboeker/6177ea03201c94ac188ef8c3dede970a)
- [sashabaranov/go-openai](https://github.com/sashabaranov/go-openai)
- [go.mau.fi/whatsmeow](https://pkg.go.dev/go.mau.fi/whatsmeow)
