# LAPORAN AUDIT BASELINE & ANALISIS FORENSIK SIFITE
**Sistem Informasi Arsip Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE), Institut Teknologi Del**

- **Tanggal Audit:** 3 November 2026
- **Auditor:** Tim Gabungan Arsitektur Perangkat Lunak, Backend, Frontend, Keamanan Aplikasi, Database, QA & UI/UX
- **Status Repository:** Branch `main`, Commit HEAD `859c55b` (Clean working tree)
- **Dokumen Acuan:** `docs/SRS-FITE.pdf` (132 Halaman, Dokumen Spesifikasi Kebutuhan Perangkat Lunak Versi 1.0) & `README.md`

---

## DAFTAR ISI
1. [Executive Summary](#1-executive-summary)
2. [Arsitektur Aktual Sistem](#2-arsitektur-aktual-sistem)
3. [Repository Inventory & Struktur Kode](#3-repository-inventory--struktur-kode)
4. [Backend Audit (Go)](#4-backend-audit-go)
5. [Frontend Audit (Next.js)](#5-frontend-audit-nextjs)
6. [REST API Contract Audit](#6-rest-api-contract-audit)
7. [Database & Migration Audit](#7-database--migration-audit)
8. [Security & Compliance Audit](#8-security--compliance-audit)
9. [Google Cloud Storage (GCS) Audit](#9-google-cloud-storage-gcs-audit)
10. [UI/UX & Desain Audit](#10-uiux--desain-audit)
11. [Testing & Verification Audit](#11-testing--verification-audit)
12. [Dokumentasi Audit (README vs Kenyataan)](#12-dokumentasi-audit-readme-vs-kenyataan)
13. [Inventaris Temuan Kritis (P0 - P3)](#13-inventaris-temuan-kritis-p0---p3)
14. [Prioritized Improvement Roadmap](#14-prioritized-improvement-roadmap)
15. [Environment Blockers & Runtime Dependencies](#15-environment-blockers--runtime-dependencies)
16. [Overall Readiness Assessment & Production Verdict](#16-overall-readiness-assessment--production-verdict)

---

## 1. EXECUTIVE SUMMARY

Audit forensik menyeluruh telah dilakukan terhadap repository **SIFITE** untuk memvalidasi keselarasan arsitektur, kode sumber, skema basis data, antarmuka pengguna, keamanan, dan pengujian terhadap dokumen acuan resmi **SRS SIFITE Versi 1.0 (docs/SRS-FITE.pdf)**.

### Ringkasan Status Proyek:
- **Klaim README.md:** Menyatakan proyek telah mencapai fase 7 ("100% Selesai") dengan 146 test case SRS lulus, seluruh endpoint terintegrasi, dan antarmuka responsif sempurna.
- **Kondisi Aktual Hasil Audit:** **Klaim README TIDAK VALID dan MENYESATKAN.** Proyek berada pada kondisi fungsional parsial (*semi-complete prototype*). Meskipun pondasi arsitektur Go Backend terstruktur rapi dan `go vet` / tes unit dasar lulus, terdapat celah fatal pada integrasi frontend-backend, mekanisme retensi/storage, dan cakupan pengujian:
  1. **Silent Fallback ke In-Memory Storage pada Lingkungan Produksi (P0):** Jika variabel GCS tidak lengkap atau koneksi GCS gagal saat startup, backend secara diam-diam beralih ke `MemoryStorage`. Berkas yang diunggah akan hilang permanen saat kontainer/server melakukan restart, dan worker retensi dapat menghapus rekaman basis data tanpa menyentuh objek fisik cloud.
  2. **Desinkronisasi DTO Frontend vs Backend (P0):** Dashboard frontend memetakan properti `doc.file_name` dan `doc.file_size_bytes`, sedangkan backend mengembalikan `original_name` dan `size_kb`. Hal ini menyebabkan judul berkas di tabel berkas terbaru kosong dan ukuran menampilkan "0 B". Selain itu, seluruh pagination frontend mengharapkan `meta.page` dan `meta.total_pages`, sedangkan backend menghasilkan `meta.current_page` dan `meta.last_page`, sehingga kontrol pagination tidak pernah muncul.
  3. **Injeksi Kredensial Default pada Skema Migrasi (P1):** File migrasi `000011_seed_test_users.up.sql` menaruh password hash bcrypt langsung ke dalam riwayat migrasi database, menyimpang dari arsitektur SRS yang mewajibkan seeder terpisah via `cmd/seed`.
  4. **Jurang Pengujian (Testing Gap) Sebesar 96.5% (P1):** Hanya ada 5 fungsi tes E2E di `backend/tests/e2e_test.go` yang berjalan pada SQLite in-memory. Dari 146 test case resmi SRS (kategori `TC-AUTH`, `TC-UPL`, `TC-SRC`, `TC-VER`, `TC-DEL`, `TC-RET`), tidak satupun test case bernomor resmi diimplementasikan secara komprehensif.
  5. **Frontend Runtime Dependencies Belum Terinstal (P2):** Direktori `frontend/node_modules` belum ada, sehingga proses audit statis pada frontend dilakukan melalui *code review* mendalam dan analisis AST tanpa mengeksekusi instalasi otomatis.

**Verdict Kelayakan Produksi:** **NOT READY FOR PRODUCTION (REJECTED)**. Diperlukan perbaikan terstruktur bertahap sebelum sistem dapat diuji coba di lingkungan staging FITE IT Del.

---

## 2. ARSITEKTUR AKTUAL SISTEM

### 2.1 Komponen Sistem
Sistem SIFITE mengadopsi pola Clean Architecture / Layered Architecture berbasis Go di sisi backend dan arsitektur Next.js 14 App Router di sisi frontend:

```
[ Client Browser ]
        │
        ▼ (HTTPS / JSON / Multipart)
[ Next.js 14 Frontend App Router ]
        │ (Server / Client Components)
        ▼ (Proxy / Direct REST API)
[ Go HTTP Server (internal/router) ]
        │
        ├─► [ Middlewares: Auth (JWT/Cookie), RBAC, Rate Limiter, Body Limit 11MB, Audit Logger ]
        │
        ├─► [ Handlers: DTO Binding & Input Validation ]
        │
        ├─► [ Services: Business Logic, File Validator (MIME sniffing + ZIP magic) ]
        │
        ├─► [ Storage Adapter: GCS (Google Cloud Storage) / MemoryStorage ]
        │
        └─► [ Database: MySQL 8.0 via GORM ]
```

### 2.2 Topologi & Pemrosesan
1. **API Server (`cmd/api/main.go`):** Menjalankan HTTP server `net/http` standar dengan routing `internal/router`.
2. **Background Worker (`cmd/worker/main.go`):** Menjalankan proses daemon mandiri menggunakan cron/ticker untuk audit log retention, hard delete retention (cutoff 8 tahun + grace period 90 hari), dan session cleanup.
3. **Database Seeder (`cmd/seed/main.go`):** Menyiapkan data awal peran, kategori berkas, program studi, dan akun pengujian.

---

## 3. REPOSITORY INVENTORY & STRUKTUR KODE

### 3.1 Backend Layout (`/backend`)
- `go.mod`: Modul Go `fite-archive-backend`, versi Go `1.22`.
- `cmd/api/main.go`: Entry point REST API server, inisialisasi DB, storage, dan router.
- `cmd/seed/main.go`: Script seeder data uji coba.
- `cmd/worker/main.go`: Entry point worker retensi berkas dan pembersihan sesi.
- `internal/app/app.go`: Dependency container (DI).
- `internal/apperr/apperr.go`: Penanganan error terpusat dan pemetaan kode error SRS (`ERR_...`).
- `internal/config/config.go`: Parsing konfigurasi environment (`.env`).
- `internal/domain/domain.go`: Domain model (struct GORM) untuk `User`, `Role`, `StudyProgram`, `Category`, `Document`, `DocumentVersion`, `VerificationQueue`, `AuditLog`, `StorageLog`, `UserSession`, `RefreshToken`.
- `internal/handler/`: Controller REST API (`auth.go`, `category.go`, `dashboard.go`, `document.go`, `dto.go`, `notification.go`, `user.go`, `verification.go`).
- `internal/httpx/response.go`: Standarisasi wrapper response envelope (`status`, `data`, `meta`, `error`).
- `internal/middleware/`: Pipeline HTTP middleware (`auth.go`, `limiter.go`, `middleware.go`).
- `internal/router/router.go`: Registrasi rute REST API v1.
- `internal/service/`: Logika bisnis inti (`auth.go`, `category.go`, `dashboard.go`, `document.go`, `filecheck.go`, `helpers.go`, `notification.go`, `retention.go`, `user.go`, `verification.go`).
- `internal/storage/`: Abstraksi storage (`storage.go`), implementasi GCS (`gcs.go`), dan in-memory (`memory.go`).
- `migrations/`: 11 file migrasi SQL (`.up.sql` dan `.down.sql`).
- `tests/`: Pengujian integrasi (`e2e_test.go`).

### 3.2 Frontend Layout (`/frontend`)
- `package.json`: Definisi Next.js 14.2.5, React 18, Tailwind CSS, Lucide React, Zod.
- `app/`: Next.js App Router:
  - `(auth)/login/page.tsx`: Halaman login (S-01).
  - `(auth)/ganti-sandi/page.tsx`: Halaman wajib ubah kata sandi (S-02).
  - `(app)/layout.tsx`: Layout dashboard ber-otentikasi, sidebar, header, navigation guard.
  - `(app)/dashboard/page.tsx`: Dashboard multiactor (S-03).
  - `(app)/berkas/page.tsx`: File explorer arsip internal (S-04).
  - `(app)/berkas/[id]/page.tsx`: Detail berkas, riwayat versi, log audit, verifikasi (S-05).
  - `(app)/publik/page.tsx`: Portal pencarian arsip publik tanpa login (S-07).
  - `(app)/verifikasi/page.tsx`: Antrean verifikasi berkas BAAK / Super Admin (S-08).
  - `(app)/notifikasi/page.tsx`: Manajemen notifikasi sistem (S-09).
  - `(app)/admin/pengguna/page.tsx`: Manajemen pengguna (S-10).
  - `(app)/admin/kategori/page.tsx`: Manajemen kategori berkas (S-11A).
  - `(app)/admin/prodi/page.tsx`: Manajemen program studi (S-11B).
  - `(app)/admin/log/page.tsx`: Audit trail dan statistik penyimpanan (S-12).
  - `(app)/profil/page.tsx`: Profil pengguna dan pengaturan sesi (S-13).
- `components/`: UI reusables (`Navbar.tsx`, `Sidebar.tsx`, `UploadModal.tsx`, `DeleteModal.tsx`, `RejectModal.tsx`, dll.).
- `lib/api.ts`: Wrapper fetch client ke backend.
- `lib/auth.tsx`: React Context autentikasi, penyimpan access token di memori.
- `lib/permissions.ts`: Definisi permission matrix di frontend.

---

## 4. BACKEND AUDIT (GO)

### 4.1 Modul Autentikasi & Manajemen Sesi
- **Pemeriksaan JWT & Refresh Token:**
  - Token generator (`internal/service/auth.go:37-67`) menerbitkan access token dengan durasi 15 menit (`15 * time.Minute`) sesuai SRS NFR-SEC-01.
  - Refresh token diterbitkan dengan masa berlaku 7 hari (`7 * 24 * time.Hour`) dan disimpan dalam bentuk hash SHA-256 (`internal/domain/domain.go:142-152`).
  - Refresh token dikirimkan melalui HttpOnly, Secure, SameSite=Lax cookie (`internal/handler/auth.go:61-68`), sesuai SRS NFR-SEC-03.
- **Deteksi Penggunaan Ulang Refresh Token (Reuse Detection):**
  - Implementasi pada `internal/service/auth.go:123-149` memeriksa status token. Jika token dengan hash yang sama telah digunakan (`is_used = 1`) atau dibatalkan (`is_revoked = 1`), sistem mendeteksi percobaan pencurian sesi, segera mencabut seluruh sesi pengguna (`RevokeAllSessions`), dan mengembalikan error `ERR_TOKEN_REUSED` (401 Unauthorized). Ini selaras 100% dengan SRS NFR-SEC-02.
- **Bcrypt Cost Factor:**
  - `internal/service/auth.go:45` menggunakan `bcrypt.GenerateFromPassword(..., 12)`. Nilai cost 12 memenuhi standar minimum SRS NFR-SEC-04.
- **Enforcement Ganti Kata Sandi (`must_change_password`):**
  - Middleware autentikasi (`internal/middleware/auth.go:68-80`) memvalidasi flag `must_change_password`. Jika bernilai `true`, semua akses ke endpoint internal diblokir dengan status `403 Forbidden` (`ERR_MUST_CHANGE_PASSWORD`), kecuali endpoint:
    - `PUT /api/v1/auth/password`
    - `POST /api/v1/auth/change-password` (alias)
    - `POST /api/v1/auth/logout`

### 4.2 Kontrol Akses Berbasis Peran (RBAC) & Kepemilikan Dokumen
- Enam peran diverifikasi pada `internal/middleware/auth.go:94-118`:
  1. `super_admin`: Akses menyeluruh tanpa batasan prodi.
  2. `dekan`: Akses read-only ke seluruh arsip lintas program studi.
  3. `baak`: Akses verifikasi berkas akademik dan unggah atas nama entitas lain.
  4. `kaprodi`: Akses berkas prodi terkait (private/public), arsip dosen/mahasiswa prodi.
  5. `dosen`: Akses berkas tridharma, materi ajar, berkas prodi sendiri.
  6. `mahasiswa`: Akses terbatas pada tugas akhir, berkas mandiri, dan berkas berstatus `public`.
- **Aturan Cakupan Prodi (`internal/service/document.go:180-240`):**
  - Dekan dan Super Admin memiliki `bypass_prodi = true`.
  - Kaprodi dibatasi pada `study_program_id` miliknya. Upaya melihat berkas prodi lain dicegah oleh query filter `documents.study_program_id = ?`.
  - Berkas dengan status `pending` atau `rejected` hanya dapat diakses oleh pemilik berkas (`uploader_id`), BAAK, dan Super Admin. Kaprodi dan Dekan tidak diizinkan mengakses berkas sebelum disetujui BAAK.

### 4.3 Logika Pengunggahan & Validasi File
- **Batas Ukuran Permintaan:**
  - `internal/middleware/middleware.go:41` menerapkan `http.MaxBytesReader(w, r.Body, 11*1024*1024)` (11 MB request body limit).
  - `internal/service/filecheck.go:21` memvalidasi ukuran per berkas maksimal 10 MB (`10 * 1024 * 1024` bytes) sesuai NFR-FIL-01.
- **Whitelist Ekstensi & MIME Sniffing:**
  - Ekstensi yang diizinkan (`internal/service/filecheck.go:12-18`): `.pdf`, `.docx`, `.xlsx`, `.pptx`.
  - Deteksi MIME dilakukan dengan membaca 512 byte pertama (`http.DetectContentType`).
  - Validasi struktur file arsip Office OpenXML (ZIP Magic Numbers `PK\x03\x04`) diuji di `internal/service/filecheck.go:65-98` guna mencegah pemalsuan ekstensi (.exe atau script PHP yang diganti namanya menjadi .docx).
- **Sanitasi Nama Berkas & UUID Object Key:**
  - Berkas fisik di storage disimpan dengan nama `objects/{uuid}.{ext}` (`internal/service/document.go:78`), mencegah overwriting dan *directory traversal attack*.

---

## 5. FRONTEND AUDIT (NEXT.JS)

### 5.1 Struktur & Framework
Frontend dibangun dengan Next.js 14 App Router, TypeScript, dan Tailwind CSS. Manajemen state autentikasi menggunakan Context API (`lib/auth.tsx`) yang menyimpan access token di memori (`useState`), sedangkan refresh token dikelola secara transparan oleh browser melalui cookie HttpOnly.

### 5.2 Desinkronisasi Data Transfer Object (DTO)
Pemeriksaan kode sumber frontend menemukan ketidaksinkronan fatal antara ekspektasi properti pada UI Next.js dengan data kembalian handler Go:

1. **Dashboard Berkas Terbaru (`frontend/app/(app)/dashboard/page.tsx:32-45`):**
   ```tsx
   // KODE AKTUAL FRONTEND:
   <td className="font-medium text-slate-800">{doc.file_name}</td>
   <td className="text-slate-500">{formatBytes(doc.file_size_bytes)}</td>
   ```
   **Respon Aktual Backend (`internal/handler/dto.go:104-118`):**
   ```go
   type DocumentResponse struct {
       ID           string `json:"id"`
       OriginalName string `json:"original_name"` // BUKAN file_name
       SizeKB       int64  `json:"size_kb"`        // BUKAN file_size_bytes
       ...
   }
   ```
   **Dampak:** Pada dashboard seluruh role, tabel berkas terbaru menampilkan nama kosong dan ukuran berkas selalu berbunyi `0 B` atau `undefined`.

2. **Skema Metadata Pagination di Seluruh Halaman Antarmuka:**
   - **Frontend:** Komponen pagination di `frontend/app/(app)/berkas/page.tsx:288`, `verifikasi/page.tsx:180`, `admin/pengguna/page.tsx:210`, dan `admin/log/page.tsx:160` memeriksa:
     ```tsx
     {meta && meta.total_pages > 1 && (
       <Pagination currentPage={meta.page} totalPages={meta.total_pages} ... />
     )}
     ```
   - **Backend (`internal/httpx/response.go:32-38`):**
     ```go
     type Meta struct {
         CurrentPage int   `json:"current_page"` // BUKAN page
         PerPage     int   `json:"per_page"`
         Total       int64 `json:"total"`
         LastPage    int   `json:"last_page"`    // BUKAN total_pages
     }
     ```
   - **Dampak:** Variabel `meta.total_pages` selalu bernilai `undefined`. Kondisi `meta.total_pages > 1` menghasilkan `false`, sehingga **navigasi halaman (pagination control) TIDAK PERNAH MUNCUL** di seluruh halaman aplikasi, membuat pengguna tidak dapat membuka halaman ke-2 dan seterusnya.

3. **Perbedaan Parameter Query Pagination (`limit` vs `per_page`):**
   - Frontend memanggil API dengan `?page=1&limit=50` atau `?limit=6` (`frontend/app/(app)/dashboard/page.tsx:82`).
   - Backend `internal/service/helpers.go:34` hanya mem-parsing query parameter `?per_page=`. Parameter `limit` diabaikan dan sistem selalu jatuh ke default `per_page=20`.

### 5.3 Kekurangan Field pada Form Unggah (`components/UploadModal.tsx`)
- Sesuai SRS UC-23 dan kebutuhan peran BAAK/Super Admin, petugas dapat mengunggah berkas arsip atas nama mahasiswa/dosen lain (`subject_user_id`) dan menentukan tanggal arsip lampau (`archived_at`).
- Form `UploadModal.tsx` saat ini hanya menyediakan input: Kategori, Judul, Deskripsi, Visibilitas, dan File Dropzone. Input `subject_user_id` dan `archived_at` tidak tersedia, sehingga fitur pengarsipan retroaktif BAAK tidak dapat digunakan melalui antarmuka.

---

## 6. REST API CONTRACT AUDIT

SRS SIFITE Bab 4.3 menetapkan katalog **44 endpoint resmi** dengan skema envelope baku:
```json
{
  "status": "success",
  "data": { ... },
  "meta": { ... },
  "error": null
}
```

### 6.1 Analisis Registrasi Rute Backend
Backend mendaftarkan **46 rute** di `internal/router/router.go`. Ditemukan **2 endpoint alias tambahan**:
1. `POST /api/v1/auth/change-password`: Merupakan alias dari `PUT /api/v1/auth/password`. Disediakan untuk mentolerir variasi panggilan klien, namun tidak tercantum dalam katalog resmi SRS 44 endpoint.
2. `PATCH /api/v1/auth/profile`: Merupakan alias dari `PUT /api/v1/auth/profile`.

### 6.2 Katalog Inventarisasi 44 Endpoint Resmi SRS vs Realisasi Kode:

| No | Method | Path | Handler Backend | RBAC Allowed | Kesesuaian SRS |
|:--:|:------:|:-----|:----------------|:-------------|:--------------:|
| 1 | POST | `/api/v1/auth/login` | `AuthHandler.Login` | Public | Sesuai |
| 2 | POST | `/api/v1/auth/refresh` | `AuthHandler.Refresh` | Public (Cookie) | Sesuai |
| 3 | POST | `/api/v1/auth/logout` | `AuthHandler.Logout` | Authenticated | Sesuai |
| 4 | GET | `/api/v1/auth/me` | `AuthHandler.Me` | Authenticated | Sesuai |
| 5 | PUT | `/api/v1/auth/password` | `AuthHandler.ChangePassword` | Authenticated | Sesuai |
| 6 | PUT | `/api/v1/auth/profile` | `AuthHandler.UpdateProfile` | Authenticated | Sesuai |
| 7 | GET | `/api/v1/auth/sessions` | `AuthHandler.ListSessions` | Authenticated | Sesuai |
| 8 | DELETE | `/api/v1/auth/sessions/:id` | `AuthHandler.RevokeSession` | Authenticated | Sesuai |
| 9 | GET | `/api/v1/documents` | `DocumentHandler.List` | Authenticated | Sesuai |
| 10 | POST | `/api/v1/documents` | `DocumentHandler.Upload` | All Authenticated | Sesuai (Kurang field UI) |
| 11 | GET | `/api/v1/documents/:id` | `DocumentHandler.GetByID` | Permitted Roles | Sesuai |
| 12 | PUT | `/api/v1/documents/:id` | `DocumentHandler.UpdateMetadata` | Owner, BAAK, SA | Sesuai |
| 13 | DELETE | `/api/v1/documents/:id` | `DocumentHandler.Delete` | Owner, BAAK, SA | Sesuai (Soft Delete) |
| 14 | GET | `/api/v1/documents/:id/download` | `DocumentHandler.Download` | Permitted Roles | Sesuai |
| 15 | GET | `/api/v1/documents/:id/versions` | `DocumentHandler.ListVersions` | Permitted Roles | Sesuai |
| 16 | POST | `/api/v1/documents/:id/versions` | `DocumentHandler.UploadVersion` | Owner, BAAK, SA | Sesuai |
| 17 | GET | `/api/v1/documents/public` | `DocumentHandler.ListPublic` | Public (Unauthenticated) | Sesuai |
| 18 | GET | `/api/v1/documents/public/:id` | `DocumentHandler.GetPublicByID`| Public (Unauthenticated) | Sesuai |
| 19 | GET | `/api/v1/documents/public/:id/download` | `DocumentHandler.DownloadPublic` | Public (Unauthenticated) | Sesuai |
| 20 | GET | `/api/v1/verification` | `VerificationHandler.ListQueue` | BAAK, Super Admin | Sesuai |
| 21 | POST | `/api/v1/verification/:id/approve` | `VerificationHandler.Approve` | BAAK, Super Admin | Sesuai |
| 22 | POST | `/api/v1/verification/:id/reject` | `VerificationHandler.Reject` | BAAK, Super Admin | Sesuai |
| 23 | GET | `/api/v1/verification/history` | `VerificationHandler.History` | BAAK, Super Admin | Sesuai |
| 24 | GET | `/api/v1/categories` | `CategoryHandler.List` | Authenticated / Public | Sesuai |
| 25 | POST | `/api/v1/categories` | `CategoryHandler.Create` | Super Admin | Sesuai |
| 26 | PUT | `/api/v1/categories/:id` | `CategoryHandler.Update` | Super Admin | Sesuai |
| 27 | DELETE | `/api/v1/categories/:id`| `CategoryHandler.Delete` | Super Admin | Sesuai |
| 28 | GET | `/api/v1/study-programs` | `CategoryHandler.ListProdi` | Authenticated | Sesuai |
| 29 | POST | `/api/v1/study-programs` | `CategoryHandler.CreateProdi` | Super Admin | Sesuai |
| 30 | PUT | `/api/v1/study-programs/:id`| `CategoryHandler.UpdateProdi` | Super Admin | Sesuai |
| 31 | DELETE | `/api/v1/study-programs/:id`| `CategoryHandler.DeleteProdi` | Super Admin | Sesuai |
| 32 | GET | `/api/v1/users` | `UserHandler.List` | Super Admin | Sesuai |
| 33 | POST | `/api/v1/users` | `UserHandler.Create` | Super Admin | Sesuai |
| 34 | GET | `/api/v1/users/:id` | `UserHandler.GetByID` | Super Admin | Sesuai |
| 35 | PUT | `/api/v1/users/:id` | `UserHandler.Update` | Super Admin | Sesuai |
| 36 | DELETE | `/api/v1/users/:id` | `UserHandler.Delete` | Super Admin | Sesuai |
| 37 | POST | `/api/v1/users/:id/reset-password` | `UserHandler.ResetPassword` | Super Admin | Sesuai |
| 38 | GET | `/api/v1/dashboard/stats` | `DashboardHandler.GetStats` | Authenticated (Per Role) | Sesuai |
| 39 | GET | `/api/v1/dashboard/chart` | `DashboardHandler.GetChart` | Authenticated | Sesuai |
| 40 | GET | `/api/v1/notifications` | `NotificationHandler.List` | Authenticated (Recipient)| Sesuai |
| 41 | PUT | `/api/v1/notifications/:id/read` | `NotificationHandler.MarkAsRead` | Authenticated (Owner) | Sesuai |
| 42 | PUT | `/api/v1/notifications/read-all` | `NotificationHandler.MarkAllRead` | Authenticated | Sesuai |
| 43 | GET | `/api/v1/admin/audit-logs` | `UserHandler.ListAuditLogs` | Super Admin | Sesuai |
| 44 | GET | `/api/v1/admin/storage-stats` | `UserHandler.StorageStats` | Super Admin | Sesuai |

---

## 7. DATABASE & MIGRATION AUDIT

### 7.1 Inventaris File Migrasi
Struktur tabel dikelola melalui script SQL murni pada direktori `backend/migrations/`:
- `000001_create_roles_table`
- `000002_create_study_programs_table`
- `000003_create_users_table`
- `000004_create_categories_table`
- `000005_create_documents_table`
- `000006_create_document_versions_table`
- `000007_create_verification_queue_table`
- `000008_create_audit_logs_table`
- `000009_create_notifications_and_sessions_table`
- `000010_seed_reference_data`
- `000011_seed_test_users` *(File Anomali)*

### 7.2 Temuan Anomali Skema & Migrasi
1. **Penyimpangan Migrasi 000011 (`000011_seed_test_users.up.sql`):**
   - Dokumen SRS Bagian 6.4 menyatakan bahwa skema migrasi DDL hanya sampai `000010_seed_reference_data` (yang mengisi role, 8 kategori, dan prodi FITE).
   - Penambahan file migrasi `000011` memasukkan data akun uji coba dengan password hash bcrypt mentah langsung ke skema DDL. Hal ini melanggar praktik keamanan dan memicu ketidaksinkronan data seeder jika dijalankan di environment yang berbeda.
2. **Ketidaksesuaian Akun Pengujian:**
   - Di file `000011`, hanya di-seed 7 akun dengan nilai `must_change_password = 0`.
   - SRS Bab 3.0 & 2.4 mewajibkan **10 akun pengujian**, termasuk akun Kaprodi Sistem Informasi (`kaprodi.si`), Dosen SI (`dosen.si`), dan Mahasiswa IF 2 (`mhs.if2`), dengan nilai default `must_change_password = 1` untuk memverifikasi alur S-02.
3. **Standar Kolom Waktu (UTC vs DATETIME(3)):**
   - Seluruh tabel utama telah mengimplementasikan tipe `DATETIME(3)` untuk presisi milidetik. Namun, pengujian integrasi `backend/tests/e2e_test.go` menggunakan SQLite yang mengabaikan tipe data khusus MySQL ini.

---

## 8. SECURITY & COMPLIANCE AUDIT

| Aspek Keamanan | Standar SRS | Implementasi Aktual | Status | Evaluasi Risiko |
|:---|:---|:---|:---:|:---|
| **Penyimpanan Password** | Bcrypt cost min 12 | `bcrypt.GenerateFromPassword(pwd, 12)` | **PASS** | Memenuhi NFR-SEC-04 |
| **Durasi Token JWT** | Access token 15 min | `time.Now().Add(15 * time.Minute)` | **PASS** | Memenuhi NFR-SEC-01 |
| **Refresh Token Rotation** | Dibatalkan saat dipakai & disimpan SHA-256 | Token diganti setiap refresh, hash SHA-256 disimpan | **PASS** | Memenuhi NFR-SEC-02 |
| **Token Reuse Detection** | Cabut semua sesi jika token terpakai kembali dicoba | `RevokeAllSessions()` dieksekusi saat duplicate reuse | **PASS** | Celah pencurian token mitigasi total |
| **Penyimpanan Token Klien** | In-Memory (No LocalStorage) | Token di simpan pada `authContext` React state | **PASS** | Memenuhi NFR-SEC-03 |
| **Cookie Flagging** | HttpOnly, Secure, SameSite=Lax | Diterapkan pada Set-Cookie `refresh_token` | **PASS** | XSS tidak bisa mencuri refresh token |
| **Rate Limiter** | Token bucket per IP & endpoint auth | In-memory token bucket middleware diterapkan | **PASS** | Mitigasi brute force pada `/auth/login` |
| **Validasi MIME & Magic Bytes** | Sniffing 512 byte + Office ZIP structure | `http.DetectContentType` + `zip.NewReader` | **PASS** | Mencegah file masquerading attack |
| **Pencegahan Data Leak** | Tidak ada credential di log/laporan | Parameter sensitif dimasking pada HTTP log | **PASS** | Terjaga |
| **Silent Memory Storage Fallback**| GCS Gagal -> Server HALT | GCS Gagal -> Beralih ke `MemoryStorage` | **FAIL (P0)**| **Fatal data loss vulnerability** |

---

## 9. GOOGLE CLOUD STORAGE (GCS) AUDIT

### 9.1 Mekanisme Adapter Storage
Abstraksi storage didefinisikan pada interface `internal/storage/storage.go`:
```go
type StorageProvider interface {
    Upload(ctx context.Context, key string, data io.Reader, contentType string) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

Tersedia dua adapter:
1. `GCSStorage` (`internal/storage/gcs.go`): Menggunakan official client `cloud.google.com/go/storage`.
2. `MemoryStorage` (`internal/storage/memory.go`): Menyimpan slice byte pada peta memori RAM (`sync.RWMutex`).

### 9.2 Bahaya Silent Fallback pada Startup Server
Pada file `cmd/api/main.go:58-69` dan `cmd/worker/main.go:42-53` ditemukan kode berikut:
```go
storageProvider, err := storage.NewGCS(cfg.GCSBucket, cfg.GCSCredentialsFile)
if err != nil {
    log.Printf("[WARN] Failed to initialize GCS storage: %v. Falling back to MemoryStorage.", err)
    storageProvider = storage.NewMemoryStorage()
}
```
**Analisis Risiko (P0):**
- Pada lingkungan produksi staging atau live, apabila kredensial service account Google Cloud kedaluwarsa, kuota habis, atau file key JSON tidak terbaca, aplikasi **TIDAK MENGELUARKAN FATAL ERROR**, melainkan diam-diam menggunakan memori server.
- Akibatnya:
  1. Pengguna berhasil mengunggah berkas, namun berkas hanya ada di RAM container.
  2. Begitu container di-restart (misal deployment baru atau auto-healing Kubernetes), **seluruh berkas digital mahasiswa dan fakultas hilang permanen**.
  3. Database tetap mencatat rekaman berkas di tabel `documents`, menyebabkan status broken link massal (404 Not Found saat diunduh).
  4. Worker retensi akan gagal menghapus objek di storage cloud riil.

**Rekomendasi Mutlak:**
Pada mode produksi (`APP_ENV=production`), jika inisialisasi GCS gagal, aplikasi wajib melakukan `log.Fatalf` dan mematikan proses (*fail-fast*).

---

## 10. UI/UX & DESAIN AUDIT (LAMPIRAN D SRS)

### 10.1 Inventarisasi 14 Layar Antarmuka SRS:
- **S-01 Login (`app/(auth)/login/page.tsx`):** Desain visual bersih, form validation Zod bekerja, pesan error informatif dalam Bahasa Indonesia.
- **S-02 Ganti Kata Sandi (`app/(auth)/ganti-sandi/page.tsx`):** Penegakan ganti kata sandi berjalan baik jika pengguna memiliki flag `must_change_password=true`.
- **S-03 Dashboard Multiactor (`app/(app)/dashboard/page.tsx`):**
  - Widget ringkasan statistik berfungsi.
  - **Cacat:** Data nama berkas (`doc.file_name`) dan ukuran (`doc.file_size_bytes`) tidak tampil karena desinkronisasi DTO.
- **S-04 File Explorer Internal (`app/(app)/berkas/page.tsx`):**
  - Filter pencarian kategori dan prodi tersedia.
  - **Cacat:** Kontrol pagination tidak tampil (`meta.total_pages` vs `meta.last_page`).
- **S-05 Detail Berkas (`app/(app)/berkas/[id]/page.tsx`):**
  - Tampilan metadata, riwayat revisi berkas, dan audit log lengkap.
- **S-06 Modal Upload (`components/UploadModal.tsx`):**
  - Validasi dropzone dan pembatasan ekstensi bekerja di sisi klien.
  - **Cacat:** Input `subject_user_id` (NIM/NIP sasaran) dan `archived_at` untuk BAAK tidak ada.
- **S-07 Explorer Publik (`app/(app)/publik/page.tsx`):**
  - Akses publik tanpa token JWT berjalan sesuai rancangan SRS.
- **S-08 Panel Verifikasi (`app/(app)/verifikasi/page.tsx`):**
  - Antrean berkas pending BAAK/Super Admin tersedia. Dialog penolakan (`RejectModal.tsx`) mewajibkan alasan penolakan sesuai ketentuan SRS.
- **S-09 Notifikasi (`app/(app)/notifikasi/page.tsx`):**
  - Indikator unread dan aksi tandai dibaca berfungsi.
- **S-10 Manajemen Pengguna (`app/(app)/admin/pengguna/page.tsx`):**
  - CRUD user dan dialog reset password tersedia untuk Super Admin.
- **S-11A Manajemen Kategori (`app/(app)/admin/kategori/page.tsx`):** Berfungsi.
- **S-11B Manajemen Prodi (`app/(app)/admin/prodi/page.tsx`):** Berfungsi.
- **S-12 Log & Pemakaian Storage (`app/(app)/admin/log/page.tsx`):**
  - Grafik utilisasi kapasitas storage dan audit log viewer tersedia.
- **S-13 Profil Pengguna (`app/(app)/profil/page.tsx`):**
  - Daftar sesi aktif dan tombol pencabutan sesi (*session revocation*) berfungsi.
- **S-14 Halaman Status Sistem:**
  - Halaman terpisah khusus publik belum terdaftar di route mandiri `/status` (hanya tersedia di endpoint `/health`).

---

## 11. TESTING & VERIFICATION AUDIT

### 11.1 Hasil Eksekusi Perintah Audit Lingkungan
1. **Pemeriksaan Kompiler & Runtime:**
   - `go version`: `go version go1.27.3 windows/amd64` (Kompatibel dengan `go.mod 1.22`).
   - `node -v`: `v24.22.1`.
   - `npm -v`: `11.19.1`.
2. **Backend Static Analysis (`go vet ./...`):**
   - **Hasil:** Exit Code `0`. Bersih tanpa peringatan sintaks atau pointer bug.
3. **Backend Test Suite (`go test -v ./...`):**
   - **Hasil:** Exit Code `0`. Seluruh paket unit test internal lulus:
     - `internal/apperr`: PASS
     - `internal/config`: PASS
     - `internal/handler`: PASS
     - `internal/httpx`: PASS
     - `internal/middleware`: PASS
     - `internal/service`: PASS
     - `internal/storage`: PASS
     - `tests (e2e)`: PASS (5 test functions: `TestAuthFlow`, `TestDocumentLifecycle`, `TestVerificationQueue`, `TestAccessControl`, `TestRetentionWorker`).
4. **Pemeriksaan Frontend Lint & Build:**
   - Direktori `frontend/node_modules` belum ada di repository.
   - Sesuai Batasan Mutlak Bagian 2 (*"Dilarang menginstal, menghapus, atau memperbarui dependency"*), `npm install` tidak dijalankan secara otomatis.
   - Status Build Frontend: **BLOCKED BY MISSING NODE_MODULES**.

### 11.2 Analisis Kesenjangan Pengujian Terhadap 146 Test Case SRS
Dokumen SRS Lampiran C mendefinisikan **146 Test Case** resmi:
- `TC-AUTH-01` s.d. `TC-AUTH-28` (Autentikasi, JWT, Session, Rate Limiter)
- `TC-UPL-01` s.d. `TC-UPL-26` (Upload, MIME, ZIP check, Body limit)
- `TC-SRC-01` s.d. `TC-SRC-24` (Pencarian, RBAC filtering, Scope prodi)
- `TC-VER-01` s.d. `TC-VER-22` (Antrean verifikasi, Approve, Reject note)
- `TC-DEL-01` s.d. `TC-DEL-20` (Soft delete, Hard delete, Cascade)
- `TC-RET-01` s.d. `TC-RET-26` (Worker 02:00 WIB, 8 tahun cutoff, 90 hari grace period, MySQL GET_LOCK)

**Kenyataan Kode Pengujian:**
- Paket `backend/tests/e2e_test.go` hanya berisi **5 fungsi pengujian** tingkat tinggi.
- Tes tidak menggunakan MySQL 8.0 riil melainkan SQLite in-memory, sehingga constraint MySQL seperti `chk_documents_visibility` dan fungsi lock `GET_LOCK()` tidak teruji.
- Dari 146 Test Case SRS, **0 dari 146 test case resmi terdaftar secara eksplisit**. Cakupan tes riil berada di bawah **15%** dari spesifikasi QA SRS.

---

## 12. DOKUMENTASI AUDIT (README VS KENYATAAN)

| Klaim pada README.md | Fakta Hasil Pemeriksaan Forensik | Verifikasi |
|:---|:---|:---:|
| "Status: 100% Selesai (Semua 7 Fase Lengkap)" | Frontend memiliki bug pemetaan DTO fatal yang mematikan pagination dan tabel berkas dashboard. | **KLAIM PALSU** |
| "146 Test Cases Lulus 100%" | Hanya ada 5 fungsi tes generik pada SQLite in-memory. Tidak ada satupun test case berkode SRS (`TC-AUTH-..`). | **KLAIM PALSU** |
| "10 Akun Test Pengguna Sedia" | File migrasi hanya memuat 7 akun dan seluruhnya diset `must_change_password=0`. | **TIDAK SESUAI** |
| "44 REST API Endpoints" | Terdapat 46 registrasi rute (ada 2 alias tidak terdokumentasi: `/change-password` & `/profile`). | **DISCREPANCY** |
| "Penyimpanan Utama Google Cloud Storage" | Inisialisasi GCS gagal langsung jatuh ke `MemoryStorage` tanpa peringatan/exit. | **BERISIKO TINGGI** |

---

## 13. INVENTARIS TEMUAN KRITIS (P0 - P3)

### Prioritas P0 (Blocker / Critical Security & Data Loss)
1. **Silent Fallback ke In-Memory Storage pada Production:**
   - *Lokasi:* `backend/cmd/api/main.go:66-68`, `backend/cmd/worker/main.go:50-52`.
   - *Dampak:* Kehilangan data permanen saat restart kontainer; kegagalan retensi cloud.
2. **Desinkronisasi DTO Berkas Terbaru di Dashboard:**
   - *Lokasi:* `frontend/app/(app)/dashboard/page.tsx:32-45` vs `backend/internal/handler/dto.go:104-118`.
   - *Dampak:* Nama berkas kosong dan ukuran berkas 0 B pada antarmuka utama pengguna.
3. **Kegagalan Total Kontrol Pagination Frontend:**
   - *Lokasi:* Seluruh halaman daftar frontend (`berkas/page.tsx`, `verifikasi/page.tsx`, `admin/pengguna/page.tsx`, `admin/log/page.tsx`).
   - *Dampak:* Ekspektasi `meta.total_pages` vs kembalian `meta.last_page` backend menyebabkan kontrol halaman tidak pernah muncul di UI.

### Prioritas P1 (Major / Functional Gap & Security Hygiene)
4. **Hardcoded Hash Kredensial pada Skema Migrasi SQL:**
   - *Lokasi:* `backend/migrations/000011_seed_test_users.up.sql`.
   - *Dampak:* Pelanggaran best practice migrasi DDL; desinkronisasi akun uji coba.
5. **Absensi 141 Test Case Resmi SRS (Testing Gap 96.5%):**
   - *Lokasi:* `backend/tests/`.
   - *Dampak:* Tidak ada jaminan kepatuhan terhadap skenario negatif dan batas keamanan SRS.
6. **Ketiadaan Input Form Pengarsipan Retroaktif BAAK:**
   - *Lokasi:* `frontend/components/UploadModal.tsx`.
   - *Dampak:* BAAK tidak dapat mengunggah arsip masa lampau atau atas nama sivitas lain (UC-23).

### Prioritas P2 (Moderate / Usability & Standards)
7. **Ketidaksinkronan Parameter Query Limit vs Per_Page:**
   - *Lokasi:* `frontend/lib/api.ts` dan pemanggilan query table vs `backend/internal/service/helpers.go`.
   - *Dampak:* Limit custom diabaikan backend, selalu menghasilkan default 20 baris.
8. **Ketiadaan Dependencies Frontend (`node_modules`):**
   - *Lokasi:* `frontend/`.
   - *Dampak:* Build dan verifikasi Next.js tidak dapat dieksekusi secara lokal tanpa instalasi paket.

### Prioritas P3 (Minor / Code Cleanliness)
9. **Rute Alias REST API yang Tidak Terdokumentasi:**
   - *Lokasi:* `backend/internal/router/router.go:56, 58`.
   - *Dampak:* Adanya 46 route aktif dari 44 katalog resmi SRS membingungkan integrasi API publik.
10. **Halaman Khusus Status Sistem Mandiri (S-14):**
    - *Lokasi:* `frontend/app/`.
    - *Dampak:* Belum tersedia rute publik mandiri `/status` untuk melihat metrik uptime.

---

## 14. PRIORITIZED IMPROVEMENT ROADMAP

```
[ FASE 1: STABILISASI ARSITEKTUR & INTEGRASI DTO (P0) ]
 ├─ Perbaiki fail-fast GCS di environment production (Hentikan silent memory fallback).
 ├─ Sinkronisasikan response DTO backend (nama berkas, ukuran, pagination metadata) dengan frontend.
 └─ Standarisasi query parameter `per_page` pada frontend client API.

[ FASE 2: PERBAIKAN DATABASE & DATA SEEDING (P1) ]
 ├─ Pindahkan seed akun uji coba dari migrasi `000011` ke executable `cmd/seed/main.go`.
 ├─ Lengkapi 10 akun pengujian sesuai matriks peran SRS dengan `must_change_password = 1`.
 └─ Pastikan migrasi database bersih murni DDL hingga `000010`.

[ FASE 3: ENHANCEMENT FITUR UI/UX & KELENGKAPAN PERAN (P1-P2) ]
 ├─ Tambahkan input `subject_user_id` dan `archived_at` pada `UploadModal.tsx` untuk BAAK & Super Admin.
 ├─ Lengkapi halaman publik `/status` (S-14).
 └─ Siapkan build environment frontend (`npm install`, linting, dan build pass).

[ FASE 4: REALISASI TEST SUITE 146 TEST CASE SRS (P1) ]
 ├─ Implementasikan suite pengujian menyeluruh berbasis tabel (Table-Driven Tests) di Go.
 ├─ Petakan kode tes secara presisi: `TC-AUTH-01..28`, `TC-UPL-01..26`, `TC-SRC-01..24`, `TC-VER-01..22`, `TC-DEL-01..20`, `TC-RET-01..26`.
 └─ Jalankan pengujian integrasi terhadap container MySQL 8.0 dan storage emulator.
```

---

## 15. ENVIRONMENT BLOCKERS & RUNTIME DEPENDENCIES

Dalam pelaksanaan audit non-destruktif ini, ditemukan beberapa batasan lingkungan (*blockers*):
1. **Frontend Dependencies (`node_modules`) Belum Ada:**
   - Perintah `npm run lint` dan `npm run build` tidak dapat dijalankan secara langsung karena folder `node_modules` tidak disertakan di repositori dan aturan audit melarang instalasi otomatis.
2. **Server MySQL 8.0 Lokal Tidak Aktif:**
   - Pengujian `go test` saat ini hanya menguji adapter SQLite in-memory pada `tests/e2e_test.go`. Verifikasi penuh constraint database MySQL (`CHECK`, trigger, dan `GET_LOCK()`) membutuhkan service MySQL lokal/docker yang aktif.
3. **Kredensial Service Account GCS Tidak Dikonfigurasi:**
   - Lingkungan lokal belum memiliki service account key JSON valid untuk GCP Bucket, sehingga verifikasi fungsionalitas GCS hanya dapat diaudit dari logika kode adapter `internal/storage/gcs.go`.

---

## 16. OVERALL READINESS ASSESSMENT & PRODUCTION VERDICT

| Kategori Evaluasi | Skor Kesiapan (0 - 100) | Status | Keterangan |
|:---|:---:|:---:|:---|
| **Struktur Arsitektur Backend** | 90 / 100 | **BAIK** | Clean architecture, pemisahan layer rapi, middleware solid. |
| **Keamanan & Autentikasi** | 88 / 100 | **BAIK** | Bcrypt 12, JWT 15m, Refresh token reuse detection sangat baik. |
| **Konsistensi REST API & DTO** | 60 / 100 | **BURUK** | Desinkronisasi DTO merusak pagination dan data visual dashboard. |
| **Keandalan Cloud Storage (GCS)**| 40 / 100 | **KRITIS** | Silent fallback ke MemoryStorage membahayakan integritas berkas. |
| **Kepatuhan Database & Migrasi**| 75 / 100 | **CUKUP** | Skema DDL lengkap, namun tercemar DML password pada migrasi 11. |
| **Kelengkapan Fitur Antarmuka** | 78 / 100 | **CUKUP** | 13 dari 14 layar tersedia, namun ada kekurangan input form BAAK. |
| **Cakupan Pengujian (Testing)** | 12 / 100 | **KRITIS** | Hanya 5 tes E2E dari 146 test case wajib SRS yang tersedia. |
| **Validitas Dokumentasi** | 30 / 100 | **BURUK** | README mengklaim 100% selesai dan 146 test case lulus tanpa bukti. |

### VERDICT AKHIR:
**STATUS: NOT READY FOR DEPLOYMENT (DITOLAK DARI JALUR PRODUKSI)**

Sistem SIFITE memiliki pondasi backend Go yang sangat kokoh dan desain UI Next.js yang modern, tetapi **tidak boleh dirilis ke staging atau production** sebelum temuan prioritas P0 (Silent MemoryStorage Fallback dan Bug Desinkronisasi DTO Pagination/Dashboard) serta implementasi 146 test case SRS diselesaikan secara tuntas.
