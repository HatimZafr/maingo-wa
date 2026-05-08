---
date: 2026-05-08
topic: whatsapp-agentic-chatbot
---

# WhatsApp Agentic AI Chatbot

## Summary

Chatbot WhatsApp berbasis Go + whatsmeow dengan kemampuan agentic AI (tool calling) dan sistem tool hybrid yang bisa diperluas hanya dengan menambah file. Dilengkapi session memory per nomor via SQLite dan allowlist nomor statis untuk keamanan.

---

## Problem Frame

Saat ini, mengakses informasi atau menjalankan automasi seringkali mengharuskan pengguna berpindah-pindah aplikasi — dari chat ke browser, dari browser ke terminal, dan seterusnya. WhatsApp adalah aplikasi yang paling sering terbuka, dan memiliki satu agen AI di dalamnya yang bisa mengeksekusi beragam tugas (mencari informasi, memanggil API, menjalankan perintah, menghitung) akan memangkas friksi tersebut secara signifikan.

---

## Actors

- **A1. End User**: Pemilik nomor WhatsApp yang sudah di-allowlist, berinteraksi dengan bot via chat 1:1.
- **A2. Admin**: Pihak yang men-deploy bot, mengelola config TOML (allowlist, LLM credentials), dan menambahkan file tool ke folder `tools/`.
- **A3. Bot Agent**: Sistem AI yang menerima pesan, memutuskan是否需要 tool, mengeksekusi tool, dan merespons pengguna.

---

## Key Flows

- **F1. Percakapan biasa (tanpa tool)**
  - **Trigger:** End User mengirim pesan yang tidak membutuhkan tool.
  - **Actors:** A1, A3
  - **Steps:** (1) A1 kirim pesan WhatsApp → (2) A3 terima, cek allowlist → (3) A3 ambil session history dari SQLite → (4) A3 kirim ke LLM bersama system prompt dan tool definitions → (5) LLM memutuskan tidak perlu tool → (6) LLM kembalikan respons → (7) A3 kirim balasan ke A1 → (8) A3 simpan pesan ke session history.
  - **Outcome:** A1 mendapat satu balasan pesan WhatsApp langsung dari LLM.
  - **Covered by:** R1, R2, R3, R4, R9, R11, R12

- **F2. Percakapan dengan tool calling**
  - **Trigger:** End User mengirim pesan yang membutuhkan eksekusi tool.
  - **Actors:** A1, A3
  - **Steps:** (1)-(4) sama dengan F1 → (5) LLM memutuskan perlu tool + memberikan tool call parameters → (6) A3 eksekusi tool synchronous → (7) A3 kirim hasil tool kembali ke LLM → (8) LLM menghasilkan respons final → (9) A3 kirim balasan ke A1 → (10) A3 simpan ke session history.
  - **Outcome:** A1 mendapat balasan WhatsApp berisi hasil eksekusi tool (mungkin beberapa pesan dalam satu turn).
  - **Covered by:** R1, R2, R3, R5, R6, R7, R11, R12

- **F3. Menambah tool baru**
  - **Trigger:** Admin ingin menambah kemampuan baru ke bot.
  - **Actors:** A2
  - **Steps:** (1) A2 buat file tool (YAML atau Go) → (2) A2 taruh di folder `tools/definitions/` atau `tools/custom/` → (3) A2 restart bot → (4) bot auto-load semua tool saat startup.
  - **Outcome:** Tool baru tersedia dan bisa langsung dipanggil oleh bot dalam percakapan.
  - **Covered by:** R5, R6, R7

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

---

## Acceptance Examples

- **AE1. Covers R1, R2, R4.** Diberikan nomor ter-allowlist mengirim "Halo, apa kabar?", bot merespons dengan satu pesan teks tanpa memanggil tool apa pun.
- **AE2. Covers R1, R2, R3.** Diberikan nomor ter-allowlist mengirim "Cari berita terbaru tentang AI", bot mendeteksi perlu web search tool, mengeksekusinya, dan merespons dengan hasil pencarian.
- **AE3. Covers R9.** Diberikan nomor TIDAK ter-allowlist mengirim pesan "Halo", bot mengabaikan dan tidak mengirim respons apa pun.
- **AE4. Covers R5, R7.** Diberikan Admin menambahkan file `weather.yml` ke folder tools dan merestart bot, tool cuaca tersedia dan bisa dipanggil oleh End User.
- **AE5. Covers R11, R12.** Diberikan End User mengirim dua pesan berurutan "Nama saya Budi" lalu "Siapa nama saya?", bot mengingat nama dari pesan pertama karena session history tersimpan di SQLite.

---

## Success Criteria

- End User bisa mengirim pesan WhatsApp ke nomor bot dan mendapat respons yang relevan dalam hitungan detik.
- Bot berhasil memanggil tool yang sesuai ketika pengguna meminta sesuatu di luar pengetahuan LLM.
- Admin bisa menambah tool baru hanya dengan membuat file dan merestart bot.
- Nomor di luar allowlist tidak bisa berinteraksi dengan bot.
- Session memory bertahan antar pesan — bot mengingat konteks percakapan sebelumnya.

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

## Key Decisions

- **Go + whatsmeow**: Library WhatsApp native Go yang aktif dikembangkan, mendukung Multi-Device API.
- **OpenAI-compatible API endpoint via local gateway**: Endpoint `http://localhost:20128/v1` memberikan fleksibilitas untuk berganti provider/model tanpa mengubah kode. Model utama: `oc/minimax-m2.5-free`.
- **System prompt di file terpisah**: System prompt disimpan di file teks sendiri (bukan di dalam config TOML) agar mudah diedit tanpa menyentuh config utama.
- **Hybrid tool system (YAML + Go)**: YAML untuk tool simpel dan cepat ditambah, Go interface untuk tool yang butuh logic kompleks. Keduanya auto-loaded dari folder.
- **TOML config file**: Format yang lebih mudah dibaca manusia dibanding JSON, dengan dukungan komentar bawaan.
- **SQLite untuk session**: Ringan, tanpa dependensi server database eksternal, cocok untuk single-instance bot.
- **Static allowlist**: Pendekatan keamanan paling sederhana — tidak perlu auth flow, cukup daftar nomor di config.

---

## Dependencies / Assumptions

- **whatsmeow** — library Go untuk WhatsApp Multi-Device API harus tersedia dan kompatibel.
- **LLM Provider** — Membutuhkan akses ke OpenAI-compatible API endpoint dengan dukungan tool calling / function calling.
- **WhatsApp Account** — Membutuhkan nomor WhatsApp yang akan digunakan sebagai bot, terdaftar di WhatsApp Multi-Device.
- **Go 1.21+** — Diasumsikan versi Go yang mendukung fitur yang dibutuhkan.
- Asumsi: Bot berjalan sebagai proses tunggal di satu mesin (tidak terdistribusi).

---

## Outstanding Questions

### Deferred to Planning

- **[Mempengaruhi R5, R6]** [Teknis] Struktur folder proyek dan pemisahan package.
- **[Mempengaruhi R6]** [Teknis] Definisi eksak Go interface `Tool`.
- **[Mempengaruhi R5]** [Teknis] Format eksak file YAML tool.
- **[Mempengaruhi R7]** [Teknis] Mekanisme auto-load tools (filesystem scan vs registry).
- **[Mempengaruhi R11]** [Teknis] Struktur tabel SQLite untuk session.
- **[Mempengaruhi R1]** [Needs research] Cara pairing WhatsApp Multi-Device via whatsmeow (QR code atau pairing code).
