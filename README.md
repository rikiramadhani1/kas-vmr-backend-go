# kas-vmr-backend (Go)

Backend REST API sistem kas komunitas, Go + Echo + GORM, clean
architecture (domain → repository → usecase → handler → routes).

Sudah tervalidasi: `go build ./...`, `go vet ./...`, `go test ./...`
lolos bersih di Go 1.22.

## Struktur Project

```
cmd/api/main.go           entry point: wiring semua layer + background workers + graceful shutdown
config/                   load env, koneksi Postgres (GORM), koneksi Redis
internal/domain/          model data
internal/repository/      akses data (GORM)
internal/usecase/         business logic
internal/dto/             request body + validasi
internal/middleware/      auth (JWT), role guard, activity logger, validator
internal/handler/         HTTP handler
internal/routes/          route registration
pkg/jwtutil/               sign/verify access & refresh token
pkg/tokenstore/            refresh token store di Redis
pkg/response/              envelope { meta, data }
pkg/phoneutil/              normalisasi nomor HP
pkg/imagevalidator/         cek EXIF kamera, edge density utk bukti transfer
pkg/ocr/                    preprocessing gambar + panggil binary tesseract
pkg/receiptparser/          ekstrak nominal & tanggal dari teks OCR
pkg/mailwatcher/            parser email notifikasi SeaBank + IMAP IDLE watcher
```

## Model Pembayaran (PENTING - baca sebelum migrasi dari versi lama)

Sistem pembayaran sudah **direstrukturisasi total**, gak lagi 1 baris
Payment per bulan dengan status pending/approved/rejected. Sekarang:

- **`Payment`** = 1 baris **per member**, isinya cuma cursor "lunas sampai
  bulan-tahun berapa" (`paid_until_month`, `paid_until_year`). Gak ada
  field `amount` atau `status` lagi.
- **`Transaksi`** (tabel baru) = catatan tiap uang masuk yang berhasil
  dicocokkan ke member (nominal, sumber `upload`/`email`/`admin`, tanggal
  transaksi). Ini yang jadi sumber "riwayat pembayaran".
- **Gak ada lagi alur pending/approve/reject.** Begitu transaksi
  terdeteksi (upload bukti atau email SeaBank), langsung otomatis:
  catat `Transaksi` → majukan cursor `Payment` → update `CashFlow` →
  kirim push notifikasi ke member.
- **Validasi anti-duplikat**: sebelum transaksi baru dicatat, dicek dulu
  apakah member yang sama sudah punya transaksi dengan **nominal +
  tanggal (hari) yang sama** — kalau iya, ditolak sebagai duplikat. Ini
  yang mencegah 1 transfer asli ke-hitung 2x kalau kebetulan kebaca lewat
  email **dan** di-upload manual sama membernya.

⚠️ **Kalau kamu sudah deploy versi lama dengan data asli**: `GORM
AutoMigrate` cuma nambah kolom/tabel baru, **gak bisa** hapus/ubah tipe
kolom lama. Tabel `payments` lama (kolom `amount`, `status`, `month`,
`year`) perlu di-drop manual dulu sebelum jalanin versi ini:

```sql
DROP TABLE IF EXISTS payments;
```

Kalau masih tahap development/testing (belum ada data beneran), gak
perlu khawatir, tinggal jalanin aja.

## Auto-confirm via email SeaBank

Member dicocokkan lewat **nomor rumah yang ditulis di kolom "Catatan"**
pas transfer — **bukan** dari nomor rekening pengirim, supaya tetap
kecocok walau transfer dari rekening suami/istri/teman. Setiap member
wajib selalu isi catatan dengan nomor rumahnya.

Baca `internal/usecase/autoconfirm_usecase.go` untuk detail
tokenisasi/matching-nya.

## Push Notification (Web Push / VAPID)

Setiap kali transaksi berhasil dicatat (lewat upload maupun email),
member otomatis dapat push notification "Pembayaran kas untuk N bulan
sudah tercatat, lunas sampai bulan X". Setup:

1. `npx web-push generate-vapid-keys` → isi `VAPID_PUBLIC_KEY` /
   `VAPID_PRIVATE_KEY` di `.env`.
2. FE perlu subscribe lewat `POST /api/members/push-subscribe` (ambil
   public key dulu dari `GET /api/members/push-public-key`, endpoint
   publik tanpa auth).
3. 1 member bisa punya banyak subscription (device berbeda) - semuanya
   dapat notif bersamaan.

Kalau `VAPID_PRIVATE_KEY` kosong, fitur ini otomatis nonaktif (skip
diam-diam, gak bikin error) - jadi aman dijalankan tanpa setup ini dulu.

## Reminder Iuran Otomatis

Job background yang jalan tiap hari, ngecek member yang masih nunggak,
kirim push reminder di tanggal & jam yang dikonfigurasi
(`REMINDER_DAY`, `REMINDER_HOUR`, default tanggal 5 jam 09:00 WIB).
Nonaktif secara default - set `REMINDER_ENABLED=true` buat aktifin.

## Menjalankan

1. `cp .env.example .env` lalu isi `DATABASE_URL`, `REDIS_URL`,
   `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` (wajib).
2. Pastikan Postgres & Redis jalan.
3. Install `tesseract-ocr` + `tesseract-ocr-ind` (buat OCR bukti
   transfer): `apt install tesseract-ocr tesseract-ocr-ind`
4. `go mod download`
5. `go run ./cmd/api` - migrasi tabel jalan otomatis via GORM
   AutoMigrate saat start.

Atau Docker: `docker build -t kas-api . && docker run --env-file .env -p 3001:3001 kas-api`

## Endpoint

Semua di bawah prefix `/api`.

| Method | Path | Auth | Keterangan |
|---|---|---|---|
| POST | /auth/admin | - | Login admin |
| POST | /auth | - | Login member (phone+PIN); menolak kalau member berstatus inactive |
| POST | /auth/token | - | Refresh access token (TIDAK butuh access token valid - itu justru tujuannya) |
| POST | /auth/logout | - | Revoke 1 refresh token |
| POST | /auth/logout-all | - | Revoke semua refresh token milik user |
| GET | /auth/profile | member/admin | Profile sesuai role |
| POST | /admin/register | admin | Daftar admin baru |
| GET | /members | member/admin | Daftar member aktif |
| GET | /members/:id | admin | Detail 1 member |
| POST | /members/pin | member | Set PIN sendiri |
| POST | /members/:member_id/reset-pin | admin | Reset PIN ke default |
| GET | /members/push-public-key | - | Ambil VAPID public key |
| POST | /members/push-subscribe | member | Daftarkan device buat push notif |
| POST | /members/push-unsubscribe | member | Batalkan subscription 1 device |
| GET | /payments/count | member | Hitung tunggakan sendiri |
| GET | /payments/recent | member | 5 transaksi terakhir milik sendiri |
| GET | /payments/unpaid | admin/bendahara | List member yang menunggak |
| POST | /payments/admin-create | admin/bendahara | Catat payment manual |
| POST | /payments/proof | member | Upload bukti transfer (auto-record) |
| GET | /payments/unmatched-transfers | admin/bendahara | Transfer email yang gak cocok nomor rumah manapun |
| GET | /payments?phone=... | admin/bendahara | Riwayat transaksi by phone |
| GET | /cashflow?year=... | member/admin | Riwayat cashflow |
| GET | /cashflow/saldo?all=true | member/admin | Saldo kas |
| POST | /cashflow | admin/bendahara | Catat cashflow manual |
| GET | /analytics/wau | admin | Weekly active users |
| GET | /analytics/action | admin | Aktivitas per member |

## Perbaikan penting

1. **Bug refresh token (2 penyebab, sudah diperbaiki)**:
   - Route `/auth/token` sebelumnya butuh access token valid buat
     diakses - kontradiktif, karena endpoint ini justru dipanggil pas
     access token sudah expired. Middleware `auth` sudah dihapus dari
     route ini.
   - FE (kalau masih pakai interceptor lama) perlu cek status **401**,
     bukan 403 - itu yang dikembalikan middleware Go untuk token
     invalid/expired.
2. **Race condition & atomicity** - approve/pencatatan payment,
   pemajuan cursor, dan update cashflow semuanya dalam 1 DB transaction
   dengan row lock.
3. **Cashflow bulan depan** - kalau member bayar untuk bulan-bulan yang
   belum tiba, `created_at` cashflow-nya di-set ke tanggal 1 bulan itu
   (bukan waktu transaksi diproses), supaya pengelompokan riwayat per
   bulan tetap akurat.
4. Detail perbaikan lain (JWT secret wajib, Redis SCAN vs KEYS,
   normalisasi nomor HP, dll) ada di riwayat commit/histori chat.

## Yang belum termasuk

- **Bot WhatsApp** - tidak ada rencana ditambahkan (digantikan push
  notification + email auto-confirm).
- Model `WeeklySummary` ada di domain (mirror schema lama) tapi tidak
  dipakai di manapun.
- Tabel `log_sign_tfs` (`LogSignTf`) masih ada di migration untuk
  kompatibilitas tapi sudah tidak dipakai - dedup sekarang lewat
  `Transaksi` (nominal + tanggal), bukan hash OCR.

## Dependency non-standar & catatan replace directive di go.mod

Beberapa dependency Go (`gorm.io/*`, `golang.org/x/*`, `gopkg.in/*`)
pakai vanity import path yang di-resolve ke repo asli di GitHub lewat
`replace` directive di `go.mod`. Ini bukan fork tidak resmi - semua
menunjuk ke mirror resmi, aman dipakai seperti biasa.
