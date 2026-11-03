# SIFITE - Sistem Informasi Arsip Berkas Digital FITE

Sistem Informasi Pengelolaan dan Pengarsipan Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE) Institut Teknologi Del untuk tiga program studi: **S1 Informatika (IF)**, **S1 Sistem Informasi (SI)**, dan **S1 Teknik Elektro (TE)** sesuai dokumen spesifikasi **SRS FITE**.

---

## 🏗️ Arsitektur Teknologi

| Komponen | Teknologi | Keterangan |
|---|---|---|
| **Backend REST API** | Go 1.22+, Gin, GORM | Clean layered architecture, failure limiter, security headers, chunk streaming |
| **Worker Retensi** | Go 1.22, robfig/cron | Distributed MySQL lock (`GET_LOCK`), pembersihan 8 tahun & 90 hari |
| **Frontend Web** | Next.js 14 App Router, TypeScript, Tailwind CSS | In-memory token security (anti-XSS), seamless refresh interceptor, RBAC layout |
| **Basis Data** | MySQL 8.0.16+ via `golang-migrate` | 9 tabel relasional, constraint DATETIME(3) UTC, referensi seed (6 peran, 3 prodi, 16 kategori) |
| **Penyimpanan Berkas** | Google Cloud Storage (GCS) | Streaming 2 MB chunk, kompensasi pembatalan atomik (SLA-DI-03), mock memori untuk tes |
| **Autentikasi & Sesi** | JWT + Cookie HttpOnly Strict | Access token (15 menit), refresh token acak 32-byte (sliding 7 hari, absolut 30 hari), rotasi atomik & deteksi token reuse |

---

## 📁 Struktur Repositori

```text
.
├── backend/                      # Service Backend Go
│   ├── cmd/
│   │   ├── api/                  # Entrypoint HTTP REST API server
│   │   └── worker/               # Entrypoint Background Retention Worker
│   ├── internal/
│   │   ├── app/                  # Inisialisasi basis data & koneksi
│   │   ├── apperr/               # Katalog galat domain & kode HTTP terstandarisasi
│   │   ├── config/               # Pemuatan & validasi konfigurasi environment
│   │   ├── domain/               # Model entitas GORM & tipe khusus JSON
│   │   ├── handler/              # HTTP Request Handlers (Auth, Document, Verifikasi, dll.)
│   │   ├── httpx/                # Response envelope, custom validator, binding helper
│   │   ├── middleware/           # Auth Bearer, CORS, CSP, LimitBody, Limiter kegagalan
│   │   ├── router/               # Pendaftaran endpoint & route guards
│   │   ├── service/              # Logika bisnis (Auth, Document, Verifikasi, Retensi)
│   │   └── storage/              # Abstraksi adapter penyimpanan (GCS & Mock Memori)
│   ├── migrations/               # Migrasi skema SQL (000001 s/d 000010)
│   └── tests/                    # Automated integration & E2E test suites
├── frontend/                     # Client Frontend Next.js App Router
│   ├── app/                      # App router pages & layouts (S-01 s/d S-14)
│   ├── components/               # Komponen UI responsif & modal interaktif
│   └── lib/                      # Klien API fetch, interceptor refresh, state auth & RBAC
├── docs/                         # Dokumen SRS FITE & spesifikasi teknis
└── README.md
```

---

## 🚀 Panduan Menjalankan Sistem

### 1. Prasyarat
- Go 1.22+
- Node.js 18+ & npm
- MySQL 8.0+
- golang-migrate CLI (`migrate`)

---

### 2. Konfigurasi Backend & Migrasi Basis Data
```bash
cd backend
cp .env.example .env
# Sesuaikan DB_DSN, JWT_SECRET, dan kredensial basis data pada .env

# Jalankan migrasi basis data (000001 sampai 000010)
migrate -path migrations -database "mysql://root:password@tcp(127.0.0.1:3306)/fite_arsip" up
```

### 3. Menjalankan Backend API & Worker
```bash
# Menjalankan server REST API di localhost:8080
go run ./cmd/api

# Menjalankan worker pembersihan retensi berkas
# Mode sekali jalan (cocok untuk pengujian & CI/CD):
go run ./cmd/worker -once

# Mode daemon terjadwal (02:00 WIB setiap hari):
go run ./cmd/worker
```

### 4. Menjalankan Frontend Next.js
```bash
cd frontend
npm install
npm run dev
# Buka http://localhost:3000 pada peramban
```

---

## 🧪 Menjalankan Pengujian Otomatis

### Pengujian Backend & E2E Test Suite
```bash
cd backend

# Menjalankan seluruh pengujian (Unit & E2E)
go test -v ./...

# Menjalankan hanya pengujian integrasi E2E sesuai Bab 3 SRS (TS-AUTH, TS-UPL, TS-SRC, TS-VER, TS-DEL, TS-RET)
go test -v ./tests
```

### Pemeriksaan Tipe & Linting Frontend
```bash
cd frontend

# Pemeriksaan validitas tipe TypeScript
npx tsc --noEmit

# Linting ESLint Next.js
npm run lint

# Membangun versi produksi Next.js
npm run build
```

---

## 🛡️ Kepatuhan Hak Akses & Matriks Peran (RBAC)

1. **Super Admin**: Pengelolaan penuh berkas fakultas, persetujuan verifikasi, hard delete permanen (`?force=true`), manajemen pengguna, kategori, prodi, dan log sistem.
2. **Dekan**: Akses baca seluruh berkas fakultas dan statistik agregat.
3. **BAAK**: Akses kelola berkas operasional, peninjauan dan persetujuan/penolakan antrean berkas publik, dan pelaporan berkas 3 prodi.
4. **Kaprodi**: Akses berkas dosen se-fakultas, berkas mahasiswa prodinya, dan berkas milik pribadi.
5. **Dosen**: Akses berkas mahasiswa prodinya dan berkas milik pribadi.
6. **Mahasiswa**: Eksklusif hanya dapat mengakses dan mengelola berkas miliknya sendiri.
