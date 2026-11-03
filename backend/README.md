# SIFITE Backend - REST API & Retention Worker

Backend service Sistem Informasi Pengelolaan dan Pengarsipan Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE) Institut Teknologi Del, dibangun menggunakan **Go 1.22+**, **Gin Web Framework**, **GORM**, dan **MySQL 8**.

---

## Arsitektur & Spesifikasi

- **REST API Server:** Arsitektur modular dengan Gin, error handling terstandarisasi ([`apperr`](internal/apperr/apperr.go) & [`httpx`](internal/httpx/httpx.go)), dan middleware keamanan (CORS, CSP/Security Headers, Failure Rate Limiter, Limit Body 11MB).
- **Autentikasi & Sesi:** JWT access token (15 menit), refresh token acak 32-byte (sliding 7 hari, absolut 30 hari) tersimpan via cookie `HttpOnly; SameSite=Strict`, rotasi token atomik, dan deteksi token reuse yang otomatis mencabut sesi saat terjadi pencurian.
- **Penyimpanan Cloud:** Abstraksi penyimpanan berkas Google Cloud Storage (GCS) dengan streaming chunk 2 MB, kompensasi pembatalan atomik (SLA-DI-03), dan memory storage mock untuk pengujian.
- **Worker Pembersihan Retensi:** Cron worker otomatis terjadwal pukul 02:00 WIB (`0 2 * * *`) menggunakan distributed lock MySQL (`GET_LOCK`) untuk keamanan multi-instans:
  - Tahap 1: Soft delete berkas arsip > 8 tahun kalender.
  - Tahap 2: Hard delete permanen berkas soft-deleted $\ge$ 90 hari.
  - Pembersihan sesi kedaluwarsa > 30 hari.

---

## Prasyarat Lingkungan

- **Go:** 1.22 atau lebih baru
- **MySQL:** 8.0.16 atau lebih baru
- **golang-migrate CLI:** v4.17+

---

## Konfigurasi Lingkungan (`.env`)

Salin file contoh konfigurasi dan sesuaikan nilai kredensial:
```bash
cp .env.example .env
```

Variabel utama yang didukung:
- `APP_ENV`: `development` | `production`
- `HTTP_ADDR`: port listener API (default `:8080`)
- `DB_DSN`: format DSN MySQL (`user:password@tcp(127.0.0.1:3306)/fite_arsip?charset=utf8mb4&parseTime=true&loc=UTC&time_zone='%2B00:00'`)
- `JWT_SECRET`: kunci rahasia minimal 32 byte
- `ALLOWED_ORIGIN`: URL origin frontend (misal `http://localhost:3000`)
- `GCS_BUCKET`: nama bucket GCS untuk berkas arsip
- `GCS_CREDENTIALS_FILE`: path ke file service account Google Cloud

---

## Migrasi Basis Data (`golang-migrate`)

Jalankan seluruh migrasi skema tabel (000001 s/d 000010) dan data referensi awal:
```bash
# Menjalankan migrasi UP
migrate -path migrations -database "mysql://$DB_DSN" up

# Membatalkan migrasi DOWN
migrate -path migrations -database "mysql://$DB_DSN" down
```

---

## Menjalankan Server & Worker

### 1. Menjalankan Server REST API
```bash
go run ./cmd/api
```
Server akan aktif di `http://localhost:8080` (health check: `/healthz`).

### 2. Menjalankan Worker Retensi
```bash
# Mode Sekali Jalan (Sangat berguna untuk pengujian / cron OS)
go run ./cmd/worker -once

# Mode Daemon (Cron terjadwal setiap hari 02:00 WIB)
go run ./cmd/worker
```

---

## Menjalankan Pengujian Otomatis (Automated Test Suite)

Jalankan seluruh pengujian unit, RBAC, dan integrasi end-to-end:
```bash
# Menjalankan seluruh test suite
go test -v ./...

# Menjalankan paket integrasi E2E secara spesifik
go test -v ./tests
```

