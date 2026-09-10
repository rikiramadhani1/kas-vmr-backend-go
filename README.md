# kas-vmr-backend (Go rewrite)

Rewrite dari sistem kas Node.js/TypeScript (Express + Prisma + Baileys) ke
**Go + Echo + GORM**, dengan clean architecture (domain → repository →
usecase → handler → routes), fase ini fokus ke REST API saja (bot
WhatsApp menyusul di fase berikutnya).

Sudah tervalidasi: `go build ./...` dan `go vet ./...` lolos bersih di
Go 1.22.

## Struktur Project

```
cmd/api/main.go           entry point: wiring config, DB, redis, semua layer, graceful shutdown
config/                   load env, koneksi Postgres (GORM), koneksi Redis
internal/domain/          model (mirror 1:1 dari schema.prisma)
internal/repository/      akses data (GORM), termasuk operasi transaksional
internal/usecase/         business logic (setara *.service.ts)
internal/dto/             request body + validasi (pengganti Zod)
internal/middleware/      auth (JWT), role guard, activity logger, validator
internal/handler/         HTTP handler (setara *.controller.ts)
internal/routes/          route registration (setara routes/*.js)
pkg/jwtutil/              sign/verify access & refresh token
pkg/tokenstore/           refresh token store di Redis (fix: SCAN, bukan KEYS)
pkg/response/             envelope { meta, data } (setara utils/response.ts)
pkg/phoneutil/            satu aturan normalisasi nomor HP (dulu ada 2 versi beda)
pkg/imagevalidator/       cek EXIF kamera, dimensi, edge density utk bukti transfer
pkg/ocr/                  preprocessing gambar (stdlib) + panggil binary tesseract
pkg/receiptparser/        ekstrak nominal & buat signature hash dari teks OCR
pkg/paymentcode/          skema "nominal unik" - encode/decode kode member dari nominal transfer
pkg/mailwatcher/          parser email notifikasi SeaBank + IMAP IDLE watcher (auto-confirm)
```

## Menjalankan

1. `cp .env.example .env` lalu isi `DATABASE_URL`, `REDIS_URL`,
   `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` (**wajib**, app akan
   langsung gagal start kalau kosong — lihat bagian "Perbaikan" #6).
2. Pastikan Postgres & Redis jalan.
3. Install `tesseract-ocr` + `tesseract-ocr-ind` di server/host (dipanggil
   via `exec.Command`, bukan cgo binding — sesuai preferensi kamu).
   - Ubuntu/Debian: `apt install tesseract-ocr tesseract-ocr-ind`
4. `go mod download`
5. `go run ./cmd/api` — migrasi tabel jalan otomatis via GORM AutoMigrate
   saat start.

Atau pakai Docker: `docker build -t kas-api . && docker run --env-file .env -p 3001:3001 kas-api`
(image sudah termasuk `tesseract-ocr`).

## Endpoint

Semua di bawah prefix `/api`.

| Method | Path | Auth | Keterangan |
|---|---|---|---|
| POST | /auth/admin | - | Login admin |
| POST | /auth | - | Login member (phone+PIN) |
| POST | /auth/token | member/admin | Refresh access token |
| POST | /auth/logout | - | Revoke 1 refresh token |
| POST | /auth/logout-all | - | Revoke semua refresh token milik user |
| GET | /auth/profile | member/admin | Profile sesuai role |
| POST | /admin/register | admin | Daftar admin baru |
| GET | /members | member/admin | Daftar member aktif |
| GET | /members/:id | admin | Detail 1 member |
| POST | /members/pin | member | Set PIN sendiri |
| POST | /members/:member_id/reset-pin | admin | Reset PIN ke default |
| POST | /payments/request | member | Ajukan bayar N bulan (pending) |
| GET | /payments/count | member | Hitung tunggakan sendiri |
| GET | /payments/pending | admin/bendahara | List payment pending |
| GET | /payments/unpaid | admin/bendahara | List member yang menunggak |
| POST | /payments/:id/approve | admin/bendahara | Approve payment |
| POST | /payments/:id/reject | admin/bendahara | Reject payment |
| POST | /payments/admin-create | admin/bendahara | Catat payment manual (approved langsung) |
| POST | /payments/proof | member | Upload bukti transfer (OCR auto-approve) |
| GET | /payments/unmatched-transfers | admin/bendahara | Transfer via email yang gak cocok kode unik manapun |
| GET | /payments?phone=... | admin/bendahara | Riwayat payment by phone |
| GET | /cashflow?year=... | member/admin | Riwayat cashflow |
| GET | /cashflow/saldo?all=true | member/admin | Saldo kas |
| POST | /cashflow | admin/bendahara | Catat cashflow manual |
| GET | /analytics/wau | admin | Weekly active users |
| GET | /analytics/action | admin | Aktivitas per member |

**Catatan asumsi:** file route asli (`payment.route.js`) yang kamu kirim
cuma mendaftarkan `/request`, `/count`, `/pending`, `/:id/approve`,
`/:id/reject`, `GET /`. Tapi controller-nya juga punya
`createPaymentByAdminHandler`, `createPaymentByProofHandler`, dan
`listUnpaidMembersHandler` yang tidak pernah di-wire ke route manapun di
file yang kamu kasih. Aku asumsikan itu memang belum sempat didaftarkan
(bukan sengaja dihilangkan) dan menambahkan route yang masuk akal untuk
ketiganya (`/payments/admin-create`, `/payments/proof`,
`/payments/unpaid`). Begitu juga role `bendahara` untuk endpoint approval
— di kode asli tidak terlihat middleware role spesifik di rute payment,
jadi aku set `admin` + `bendahara` sebagai default yang masuk akal.
Silakan sesuaikan di `internal/routes/routes.go` kalau beda dari yang kamu mau.

## Fitur baru: Auto-confirm payment via email notifikasi SeaBank

Fitur ini **tidak ada** di kode Node.js asli — ditambahkan berdasarkan
diskusi kita soal cara paling simple buat konfirmasi pembayaran tanpa
OCR/API bank. Cara kerjanya:

1. Setiap member ditugasin transfer dengan **nominal unik**:
   `n_bulan * IURAN_AMOUNT + kode_unik_member` (kode unik = ID member).
   Contoh: iuran Rp20.000, member ID 7 bayar 1 bulan → transfer **Rp20.007**.
   Lihat `pkg/paymentcode` (`Encode`/`Decode`), sudah ada unit test.
2. Email notifikasi "transfer masuk" dari SeaBank yang masuk ke Gmail kas
   di-watch via **IMAP IDLE** (near-real-time, bukan polling berat) —
   lihat `pkg/mailwatcher`. Fallback otomatis ke polling kalau server
   gak dukung IDLE.
3. Isi email di-parse (line-based, sudah divalidasi pakai contoh email
   asli SeaBank kamu, termasuk edge case field `Catatan` yang kosong).
4. Nominal hasil parse di-*decode* balik jadi `(memberID, jumlah_bulan)`.
   Kalau cocok member aktif → otomatis panggil `CreatePaymentByAdmin`
   (payment langsung `approved` + cashflow ke-update, atomic).
   Kalau nominal gak cocok skema kode unik manapun (misal donasi/salah
   transfer) → dicatat sebagai `unmatched` di tabel
   `email_transaction_logs`, bisa dicek admin lewat
   `GET /payments/unmatched-transfers` buat diproses manual.
5. Dedup pakai `No. Referensi` dari email (bukan IMAP `\Seen` flag) —
   jadi aman kalau ada reprocessing/restart, gak bakal dobel-catat payment
   yang sama.

**Cara aktifkan:** set `MAIL_WATCHER_ENABLED=true` + isi `MAIL_USERNAME`
(Gmail kas) dan `MAIL_APP_PASSWORD` (generate dari Google Account →
Security → App Passwords, **bukan** password Gmail biasa — akun Gmail
wajib 2FA aktif dulu). Default nonaktif (`false`), REST API tetap jalan
normal tanpa ini.

**Batasan yang perlu kamu tau:**
- Parser cocok ke format email SeaBank yang kamu kasih sebagai contoh —
  kalau SeaBank ubah template email, parser bisa perlu disesuaikan
  (`pkg/mailwatcher/parser.go`, sudah ada test-nya buat pegangan).
- Skema nominal unik butuh member **transfer manual dengan nominal yang
  benar** (bukan sekadar Rp20.000 polos) — perlu edukasi ke member soal
  ini (misal ditampilkan di halaman "cara bayar" member).
- `UNIQUE_CODE_BASE` (default 1000, dukung sampai 999 member) wajib bisa
  membagi habis `IURAN_AMOUNT` — divalidasi saat startup, app gak akan
  jalan kalau kombinasinya gak valid.

## Perbaikan dari versi Node.js

1. **Race condition ID cash flow** — versi lama pakai
   `aggregate(max(id)) + 1` manual sebelum insert. Sekarang pakai
   auto-increment Postgres.
2. **Approve payment + update cashflow sekarang atomic** — dibungkus 1
   DB transaction dengan row lock (`SELECT ... FOR UPDATE`), jadi dua
   approval bersamaan untuk bulan yang sama tidak akan saling menimpa
   (lost update). Lihat `PaymentUsecase.ApprovePayment` &
   `CashFlowRepository.UpsertByDescription`.
3. **Perhitungan tunggakan disatukan** — dulu ada 3 implementasi beda
   (kasRepository vs payment.service, dua-duanya beda hasil kalau ada
   bulan yang di-skip). Sekarang cuma 1 fungsi
   (`usecase.CalculateUnpaidMonths`) dipakai di semua tempat.
4. **Hardcode `member_id > 15` dihapus** — diganti pengecekan member
   benar-benar ada & aktif di database.
5. **JWT secret wajib di-set** — tidak ada fallback ke string hardcoded
   seperti versi lama (`'access_secret'`). App gagal start kalau env
   kosong.
6. **`RevokeAll` refresh token pakai `SCAN`**, bukan `KEYS` (yang
   blocking dan berbahaya di Redis production dengan banyak key).
7. **Nama bendahara untuk validasi OCR jadi configurable**
   (`BENDAHARA_NAME_KEYWORD`), bukan string hardcoded di source.
8. **Normalisasi nomor HP disatukan** jadi 1 fungsi (`pkg/phoneutil`),
   dulu ada 2 aturan beda di controller vs repository yang berisiko
   out-of-sync.
9. **Validasi upload file bukti transfer** — max 10MB + whitelist
   ekstensi (jpg/jpeg/png) sebelum diproses; file sementara selalu
   dihapus setelah selesai diproses (versi lama meng-comment baris hapus
   file, jadi folder upload numpuk terus).
10. **Pesan error ke client disaring** — error internal tak terduga
    dibalas pesan generik ("Terjadi kesalahan internal"), bukan
    `err.Error()` mentah yang berisiko bocorin detail implementasi.
11. **PIN belum diset** sekarang dapat pesan jelas ("Kamu belum mengatur
    PIN, silakan hubungi admin") alih-alih "PIN tidak sesuai" yang
    membingungkan.
12. **Timestamp DB distandarkan ke UTC** (`NowFunc` di GORM config),
    kalkulasi tunggakan pakai timezone Asia/Jakarta eksplisit — konsisten
    dengan kebutuhanmu menghindari drift timezone.

## Yang belum termasuk (sengaja, sesuai kesepakatan)

- **Bot WhatsApp (Baileys/whatsmeow)** — fase 2, belum ada di rewrite ini.
- **Scheduler broadcast bulanan** (`node-cron` equivalent) — menunggu bot
  aktif dulu di fase 2.
- Model `WeeklySummary` ada di domain (mirror schema) tapi belum ada
  repository/usecase karena tidak dipakai di kode asli manapun.

## Dependency non-standar & catatan replace directive di go.mod

Beberapa dependency Go (`gorm.io/*`, `golang.org/x/*`, `gopkg.in/*`)
pakai "vanity import path" yang di-resolve ke repo asli di GitHub lewat
`replace` directive di `go.mod`. Ini **bukan fork tidak resmi** — semua
menunjuk ke mirror resmi (mis. `gorm.io/gorm` → `github.com/go-gorm/gorm`,
`golang.org/x/crypto` → `github.com/golang/crypto`), jadi aman dipakai
seperti biasa. `go mod tidy` / `go build` akan tetap bekerja normal di
environment kamu.
