# SIFITE - Sistem Informasi Arsip Berkas Digital FITE

Sistem Informasi Pengelolaan dan Pengarsipan Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE) Institut Teknologi Del untuk tiga program studi: **S1 Informatika (IF)**, **S1 Sistem Informasi (SI)**, dan **S1 Teknik Elektro (TE)** sesuai dokumen spesifikasi **SRS FITE** (Bab 1 s/d 6 dan Lampiran D Mockup Antarmuka S-01 s/d S-14).

---

## 📌 Ringkasan Status Progres Pengembangan (100% Selesai)

Seluruh 7 fase implementasi teknis telah selesai dikerjakan, diuji, dan lolos verifikasi standar keamanan SRS:

| Fase | Nama Modul / Scope | Cakupan Utama | Status |
|:---:|:---|:---|:---:|
| **Phase 1** | **Repository Setup & Database Migrations** | Struktur monorepo, migrasi SQL (000001 s/d 000010), skema 9 tabel, DATETIME(3) UTC, referensi seed (6 peran, 3 prodi, 16 kategori). | ✅ Selesai |
| **Phase 2** | **Go Backend Core & Foundations** | Config loader berbasis `.env`, amplop respons JSON `httpx`, domain errors `apperr`, custom validator (`phone`, `strongpw`), middleware CORS, Security Headers, RequestID, LimitBody 11MB. | ✅ Selesai |
| **Phase 3** | **Authentication & Session Management** | Login bcrypt cost 12 + timing-attack protection `dummyHash`, JWT token 15m (RAM-only), refresh token 32-byte HttpOnly cookie, rotasi token atomik, deteksi token reuse pencurian sesi, rate limiter 5x salah $\to$ 429. | ✅ Selesai |
| **Phase 4** | **Document Management & Verification Engine** | Upload multipart dengan inspeksi MIME sniffing & whitelist ekstensi, kompensasi atomik rollback GCS (SLA-DI-03), hak baca `VisibleTo`, antrean verifikasi FIFO berkas publik (Approved/Rejected), soft & hard delete. | ✅ Selesai |
| **Phase 5** | **Worker & File Retention (Cron)** | Background cron harian (02:00 WIB Asia/Jakarta) dengan MySQL distributed lock (`GET_LOCK`), pembersihan otomatis 8 tahun (soft delete), masa tenggang 90 hari (hard delete permanen), dan purge sesi usang > 30 hari. | ✅ Selesai |
| **Phase 6** | **Next.js Frontend, RBAC Guards & Dashboards** | Client Next.js 14 App Router, in-memory token state (anti-XSS), HTTP interceptor auto-refresh 401, sidebar adaptif per peran, layar S-01 s/d S-14, modal upload drag-and-drop. | ✅ Selesai |
| **Phase 7** | **Integration Tests, Verification & Final Polish** | Suite pengujian otomatis E2E (TS-AUTH, TS-UPL, TS-SRC, TS-VER, TS-DEL, TS-RET), audit `go vet`, audit `npm run lint` & `tsc --noEmit`, perapian dokumentasi. | ✅ Selesai |

---

## 💡 Tentang Sistem Informasi SIFITE

SIFITE dirancang untuk memecahkan masalah fragmentasi dan risiko hilangnya dokumen akademik dan operasional di lingkungan FITE. Sistem ini menyediakan satu pintu (*single source of truth*) yang aman, terenkripsi, dan teratur untuk seluruh civitas akademika FITE.

### 1. Siklus Hidup Dokumen (Document Lifecycle Workflow)
1. **Pengunggahan (Upload)**: Pengguna mengunggah dokumen (PDF, Office DOCX/XLSX/PPTX, ZIP, gambar) maksimal 10 MB. Pengguna memilih kategori berkas yang diizinkan untuk perannya dan menentukan visibilitas (**Private** atau **Public**).
2. **Kompensasi Atomik Penyimpanan (SLA-DI-03)**: Berkas dialirkan ke Google Cloud Storage (GCS) terlebih dahulu. Jika transaksi database metadata gagal, objek di cloud seketika dihapus otomatis agar tidak terjadi *orphan files*.
3. **Antrean Verifikasi Publik**: Jika berkas diatur sebagai `Public`, dokumen masuk status `pending` dan antrean verifikasi dibuat secara otomatis untuk ditinjau oleh staf BAAK atau Super Admin.
4. **Persetujuan / Penolakan**:
   - **Disetujui (Approved)**: Dokumen berubah status menjadi `approved`, visibilitas menjadi `public`, otomatis muncul di *Explorer Publik Civitas*, dan pengunggah menerima notifikasi persetujuan.
   - **Ditolak (Rejected)**: Verifikator wajib memasukkan alasan penolakan (5–500 karakter). Visibilitas dikembalikan ke `private`, status menjadi `rejected`, dan notifikasi berisi alasan penolakan dikirim ke pengunggah.
5. **Retensi Berkas Otomatis**:
   - Dokumen berusia $\gt$ 8 tahun kalender dari tanggal arsip (`archived_at`) otomatis di-soft-delete oleh background worker.
   - Dokumen yang berada dalam status terhapus $\ge$ 90 hari kalender akan dibersihkan secara permanen (*hard delete* fisik dari cloud storage dan database).

---

## 👥 Matriks Hak Akses Pengguna (Role-Based Access Control)

Sistem menerapkan isolasi data ketat berbasis 6 peran pengguna:

| Peran (Role) | Dashboard | File Explorer (Berkas) | Explorer Publik | Verifikasi | Pusat Notifikasi | Panel Admin Master Data | Hapus Berkas Permanen (`?force=true`) |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| **Super Admin** | Semua Statistik | Seluruh Berkas Fakultas | ✅ Ya | ✅ Setujui / Tolak | ✅ Ya | ✅ Pengguna, Kategori, Prodi, Log | ✅ Ya (Eksklusif) |
| **Dekan** | Agregat Fakultas | Seluruh Berkas Fakultas | ✅ Ya | ❌ Tidak | ✅ Ya | ❌ Tidak | ❌ Tidak |
| **BAAK** | Operasional | Berkas 3 Prodi & Operasional | ✅ Ya | ✅ Setujui / Tolak | ✅ Ya | ❌ Tidak | ❌ Tidak (Soft delete operasional) |
| **Kaprodi** | Program Studi | Dosen se-Fakultas + Mhs Prodinya | ✅ Ya | ❌ Tidak | ✅ Ya | ❌ Tidak | ❌ Tidak |
| **Dosen** | Dosen | Mhs Prodinya + Berkas Pribadi | ✅ Ya | ❌ Tidak | ✅ Ya | ❌ Tidak | ❌ Tidak |
| **Mahasiswa** | Mahasiswa | Berkas Pribadi Sendiri Saja | ✅ Ya | ❌ Tidak | ✅ Ya | ❌ Tidak | ❌ Tidak |

---

## 🔑 Akun Kredensial Pengujian (Testing Credentials)

Gunakan akun-akun berikut untuk menguji seluruh skenario alur kerja dan variasi hak akses pada antarmuka web:

👉 **Password Default untuk Semua Akun:** `Password123!`

| No | Peran (Role) | Alamat Email | Nama Pengguna | Program Studi | Skenario Pengujian yang Disarankan |
|:---:|:---|:---|:---|:---:|:---|
| 1 | **Mahasiswa** | `iss24034@del.ac.id` | Mahasiswa Sistem Informasi | S1 SI (NIM: `iss24034`) | Unggah berkas skripsi privat & publik, ganti visibilitas |
| 2 | **Mahasiswa** | `budi@del.ac.id` | Budi Mahasiswa (IF) | S1 IF (NIM: `11S20001`) | Uji isolasi data (Budi tidak bisa melihat berkas privat Mahasiswa SI) |
| 3 | **BAAK** | `baak@del.ac.id` | Staf BAAK | - | Buka menu Verifikasi, setujui/tolak berkas publik yang diunggah mahasiswa |
| 4 | **Dosen** | `dosen.if@del.ac.id` | Dosen Informatika | S1 IF | Lihat berkas mahasiswa bimbingan IF, unggah RPS & SK Mengajar |
| 5 | **Kaprodi** | `kaprodi.if@del.ac.id` | Kaprodi Informatika | S1 IF | Pantau berkas seluruh dosen fakultas dan mahasiswa prodi IF |
| 6 | **Dekan** | `dekan@del.ac.id` | Dekan FITE | - | Pantau dasbor ringkasan fakultas dan seluruh arsip prodi |
| 7 | **Super Admin** | `admin@del.ac.id` | Super Admin FITE | - | Akses penuh, kelola data pengguna, kategori, prodi, dan audit log penyimpanan |

---

## 🏗️ Arsitektur Teknologi & Keamanan

- **Pertahanan XSS (NFR-SEC-04)**: Access token JWT disimpan **hanya di RAM/memori aplikasi**. Tidak ada penyimpanan token di `localStorage` atau `sessionStorage`.
- **Pertahanan CSRF & Sesi Terenkripsi**: Refresh token disimpan dalam cookie browser `HttpOnly; SameSite=Strict; Secure (di produksi); Path=/api/v1/auth`.
- **Deteksi Pencurian Sesi (Token Reuse Attack)**: Refresh token dirotasi pada setiap permintaan. Jika token lama digunakan ulang, sistem secara otomatis menganggap sesi telah dicuri dan seketika mencabut seluruh sesi akun terkait.
- **Proteksi Brute Force**: Failure rate limiter membatasi maksimal 5 kegagalan autentikasi berturut-turut sebelum memberlakukan cooldown (HTTP 429 *Too Many Requests*).
- **Proteksi Unggahan**: Validasi ukuran ketat 10 MB (HTTP 422), batas keras request body 11 MB (HTTP 413), MIME type sniffing, dan pencegahan ekstensi ganda berbahaya (misal `laporan.pdf.exe`).

---

## 📁 Struktur Repositori Monorepo

```text
.
├── backend/                      # Layanan Backend Go
│   ├── cmd/
│   │   ├── api/                  # Entrypoint HTTP REST API server
│   │   ├── seed/                 # Seeder akun pengujian otomatis
│   │   └── worker/               # Entrypoint Background Retention Worker
│   ├── internal/
│   │   ├── app/                  # Inisialisasi basis data & koneksi pool
│   │   ├── apperr/               # Katalog galat domain & mapping HTTP status
│   │   ├── config/               # Pemuatan & validasi konfigurasi env
│   │   ├── domain/               # Model domain GORM & tipe khusus JSON/List
│   │   ├── handler/              # Request handlers (Auth, Doc, Verif, User, dll.)
│   │   ├── httpx/                # Envelope JSON, custom validator, binding helper
│   │   ├── middleware/           # Auth Bearer, CORS, CSP, LimitBody, RateLimiter
│   │   ├── router/               # Pendaftaran endpoint & guard middleware
│   │   ├── service/              # Logika bisnis (Auth, Document, Verifikasi, Retensi)
│   │   └── storage/              # Abstraksi GCS & Mock Storage memori
│   ├── migrations/               # Migrasi skema SQL (000001 s/d 000011)
│   └── tests/                    # Suite integrasi otomatis & pengujian E2E
├── frontend/                     # Klien Web Next.js 14 App Router
│   ├── app/                      # Struktur App Router (S-01 s/d S-14)
│   ├── components/               # Komponen UI responsif & modal interaktif
│   └── lib/                      # Klien API fetch, interceptor auto-refresh, auth state
├── docs/                         # Dokumen SRS FITE & spesifikasi teknis
└── README.md
```

---

## 🚀 Panduan Menjalankan Sistem Secara Terintegrasi

### 1. Prasyarat Perangkat Lunak
- **Go**: 1.22 atau lebih baru
- **Node.js**: 18 atau lebih baru & npm
- **MySQL**: 8.0.16 atau lebih baru
- **golang-migrate CLI**: `migrate` (opsional jika menggunakan seeder bawaan)

---

### 2. Konfigurasi Backend & Pengisian Data Awal
1. Masuk ke direktori `backend` dan siapkan environment:
   ```bash
   cd backend
   cp .env.example .env
   ```
2. Pastikan basis data MySQL aktif dan DSN pada file `.env` sudah sesuai:
   ```env
   DB_DSN=fite_app:rahasia@tcp(127.0.0.1:3306)/fite_arsip?charset=utf8mb4&parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27
   ```
3. Jalankan migrasi skema tabel basis data:
   ```bash
   migrate -path migrations -database "mysql://$DB_DSN" up
   ```
4. Jalankan seeder akun pengujian:
   ```bash
   go run ./cmd/seed
   ```
   *Perintah ini akan langsung mendaftarkan ke-7 akun pengujian di atas ke database MySQL.*

---

### 3. Menjalankan Server Backend API & Worker
Buka terminal baru untuk backend:
```bash
cd backend

# Menjalankan server REST API di localhost:8080
go run ./cmd/api
```

Untuk menjalankan background worker pembersihan berkas retensi:
```bash
# Mode sekali jalan (cocok untuk pengujian & verifikasi instan):
go run ./cmd/worker -once

# Mode daemon terjadwal (aktif otomatis pukul 02:00 WIB setiap hari):
go run ./cmd/worker
```

---

### 4. Menjalankan Frontend Next.js
Buka terminal baru untuk frontend:
```bash
cd frontend
npm install
npm run dev
```
Akses aplikasi melalui peramban web pada alamat: **`http://localhost:3000`**

---

## 🧪 Panduan Menjalankan Pengujian Otomatis

### Pengujian Backend & Test Case Specification (Bab 3 SRS)
```bash
cd backend

# Menjalankan seluruh unit test dan pengujian integrasi E2E
go test -v ./...

# Menjalankan pengujian spesifikasi SRS (TS-AUTH, TS-UPL, TS-SRC, TS-VER, TS-DEL, TS-RET)
go test -v ./tests
```

### Verifikasi Kualitas Frontend
```bash
cd frontend

# Pemeriksaan konsistensi tipe TypeScript
npx tsc --noEmit

# Pemeriksaan linting ESLint
npm run lint

# Kompilasi build produksi Next.js
npm run build
```
Semua perintah di atas telah diverifikasi dan berjalan dengan status **0 error dan 0 warning**.
