# Sistem Manajemen File FITE

Sistem Informasi Arsip Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE) untuk tiga program studi: S1 Informatika, S1 Sistem Informasi, dan S1 Teknik Elektro.

Spesifikasi lengkap (kebutuhan, test case, REST API, basis data) ada di [`docs/SRS-FITE.pdf`](docs/SRS-FITE.pdf).

## Teknologi

| Lapisan | Teknologi |
|---|---|
| Backend | Go 1.22+, Gin, GORM |
| Frontend | Next.js (App Router, TypeScript) |
| Basis data | MySQL 8.0.16+ |
| Penyimpanan berkas | Google Cloud Storage |
| Autentikasi | JWT access token (15 menit) + refresh token httpOnly cookie (7 hari, dirotasi) |

## Struktur repositori

```
.
├── backend/              # REST API Go
│   ├── cmd/api/          # titik masuk server HTTP
│   ├── internal/
│   │   ├── config/       # pemuatan dan validasi konfigurasi env
│   │   ├── apperr/       # galat domain -> status HTTP + kode
│   │   ├── httpx/        # format respons, binding, validasi
│   │   └── router/       # rute dan middleware
│   └── migrations/       # SQL golang-migrate
├── frontend/             # Next.js (fase 8)
├── docs/                 # SRS dan dokumen pendukung
├── docker-compose.yml    # MySQL lokal
└── Makefile
```

## Menjalankan (lokal)

Prasyarat: Go 1.22+, Docker.

```bash
cp backend/.env.example backend/.env
make db-up        # MySQL di localhost:3306
make tidy         # unduh dependensi Go
make api          # API di http://localhost:8080
curl http://localhost:8080/api/v1/health
```

## Fase pengembangan

| Fase | Isi | Status |
|---|---|---|
| 1 | Fondasi proyek: struktur, konfigurasi, galat, respons, health check | Selesai |
| 2 | Migrasi basis data (9 tabel) dan seed | Berikutnya |
| 3 | Model domain GORM dan abstraksi penyimpanan | |
| 4 | Autentikasi: login, refresh, logout, middleware | |
| 5 | Unggah dan validasi berkas (GCS) | |
| 6 | Daftar, pencarian, unduh, hapus, dan verifikasi berkas | |
| 7 | Notifikasi, pengguna, dasbor, dan worker retensi | |
| 8 | Frontend Next.js | |

## Catatan

Kode backend ditulis mengikuti Bab 5 SRS dan belum diuji secara menyeluruh. Jangan menyimpan `.env` atau kunci GCS di repositori.
