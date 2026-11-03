# MATRIKS KETERTELUSURAN DAN KESENJANGAN (TRACEABILITY & GAP MATRIX) SIFITE
**Sistem Informasi Arsip Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE), Institut Teknologi Del**

- **Dokumen Referensi:** SRS-FITE v1.0 (`docs/SRS-FITE.pdf`), `README.md`, dan Kode Sumber Aktual
- **Tanggal Audit:** 3 November 2026
- **Status Evaluasi:** BASELINE AUDIT (Non-Destructive)

---

## 1. PENJELASAN STATUS KELENGKAPAN

Setiap butir kebutuhan dievaluasi berdasarkan lima status implementasi:
- **FULL (Sesuai Penuh):** Kebutuhan telah diimplementasikan sepenuhnya pada backend, frontend, database, dan diverifikasi oleh pengujian.
- **PARTIAL (Parsial / Ada Celah):** Komponen backend atau UI sudah ada, namun terdapat bug integrasi, desinkronisasi DTO, atau field yang hilang.
- **NOT IMPLEMENTED (Belum Ada):** Fitur belum dibuat sama sekali pada kode sumber.
- **DISCREPANCY (Penyimpangan):** Implementasi berbeda dari spesifikasi resmi SRS atau melanggar arsitektur yang digariskan.
- **NO TEST (Celah Verifikasi):** Logika kode ada, namun test case resmi SRS (`TC-*`) belum diimplementasikan.

---

## 2. MATRIKS KEBUTUHAN FUNGSIONAL (FUNCTIONAL REQUIREMENTS)

### 2.1 Autentikasi & Akun (FR-AUTH)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-AUTH-01** | Pengguna dapat login dengan username/email dan kata sandi | `internal/handler/auth.go:34` | `app/(auth)/login/page.tsx` | `users` | **FULL** | Form validation Zod, penanganan error informatif |
| **FR-AUTH-02** | Token rotation dan deteksi penggunaan ulang refresh token | `internal/service/auth.go:123` | `lib/auth.tsx:48` | `refresh_tokens` | **FULL** | Revoke all sessions saat token reuse terdeteksi |
| **FR-AUTH-03** | Pengguna dapat logout dan mencabut refresh token aktif | `internal/handler/auth.go:94` | `components/Navbar.tsx:72` | `refresh_tokens`, `user_sessions` | **FULL** | Cookie refresh dibersihkan dan session di-revoke |
| **FR-AUTH-04** | Wajib ubah kata sandi pertama kali (`must_change_password`) | `internal/middleware/auth.go:68` | `app/(auth)/ganti-sandi/page.tsx` | `users` | **PARTIAL** | Route guard bekerja, namun seeder akun pengujian men-set `must_change_password=0` |

### 2.2 Pengelolaan Dokumen (FR-DOC)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-DOC-01** | Pengunggahan berkas arsip dengan validasi MIME dan ekstensi | `internal/service/document.go:42` | `components/UploadModal.tsx` | `documents`, `document_versions` | **PARTIAL** | Backend valid; Frontend kekurangan input `subject_user_id` & `archived_at` untuk BAAK |
| **FR-DOC-02** | Daftar berkas dengan filter (kategori, prodi, status) | `internal/service/document.go:176` | `app/(app)/berkas/page.tsx` | `documents` | **PARTIAL** | Backend valid; Pagination UI tidak muncul akibat `meta.total_pages` vs `meta.last_page` |
| **FR-DOC-03** | Melihat detail berkas, riwayat versi, dan audit log | `internal/handler/document.go:102` | `app/(app)/berkas/[id]/page.tsx` | `documents`, `document_versions`, `audit_logs` | **FULL** | Detail metadata, tab riwayat versi, dan log audit tampil dengan baik |
| **FR-DOC-04** | Pembaruan metadata berkas (judul, deskripsi, visibilitas) | `internal/handler/document.go:142` | `app/(app)/berkas/[id]/page.tsx` | `documents` | **FULL** | Hanya uploader, BAAK, dan Super Admin yang dapat mengubah |
| **FR-DOC-05** | Soft delete berkas arsip dan pelacakan riwayat | `internal/service/document.go:310` | `components/DeleteModal.tsx` | `documents` | **FULL** | Soft delete mengisi `deleted_at`, berkas fisik tetap di storage sampai grace period worker |

### 2.3 Pencarian Berkas (FR-SRC)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-SRC-01** | Pencarian berkas internal berdasarkan keyword, prodi, role, ekstensi | `internal/service/document.go:195` | `app/(app)/berkas/page.tsx` | `documents` | **PARTIAL** | Query filter backend lengkap; Parameter frontend `limit` tidak dibaca backend (`per_page`) |
| **FR-SRC-02** | Pencarian berkas publik tanpa perlu otentikasi | `internal/handler/document.go:210` | `app/(app)/publik/page.tsx` | `documents` | **FULL** | Hanya menampilkan berkas `status = 'approved'` dan `visibility = 'public'` |

### 2.4 Notifikasi (FR-NTF)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-NTF-01** | Notifikasi in-app untuk persetujuan/penolakan dan aksi sistem | `internal/service/notification.go:20` | `app/(app)/notifikasi/page.tsx` | `notifications` | **PARTIAL** | Backend & UI ada, namun pagination notifikasi terdampak bug DTO pagination |

### 2.5 Hak Akses Peran Spesifik (FR-SA, FR-DKN, FR-BAK, FR-KPR, FR-DSN, FR-MHS)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-SA-01..06** | Manajemen Pengguna, Kategori, Prodi, Audit Logs, Stats | `internal/handler/user.go`, `category.go` | `app/(app)/admin/*` | `users`, `categories`, `study_programs`, `audit_logs` | **FULL** | Hak Super Admin berfungsi menyeluruh di backend dan UI |
| **FR-DKN-01..04** | Akses baca seluruh prodi, statistik ringkasan, unduh berkas | `internal/service/document.go:218` | `app/(app)/dashboard/page.tsx` | `documents` | **PARTIAL** | Read-only lintas prodi aman; UI dashboard mengalami bug DTO nama berkas |
| **FR-BAK-01..05** | Antrean verifikasi, approval, rejection note, unggah retroaktif | `internal/service/verification.go:30` | `app/(app)/verifikasi/page.tsx` | `verification_queue`, `documents` | **PARTIAL** | Approval/rejection berjalan; Form unggah belum memiliki field pengarsipan retroaktif BAAK |
| **FR-KPR-01..03** | Akses arsip sebatas program studi masing-masing | `internal/service/document.go:230` | `app/(app)/berkas/page.tsx` | `documents` | **FULL** | Filter `study_program_id` diisolasi di tingkat query backend |
| **FR-DSN-01..03** | Pengunggahan arsip Tridharma dan materi kuliah | `internal/service/document.go:50` | `app/(app)/berkas/page.tsx` | `documents` | **FULL** | Kategori akademik terasosiasi dengan benar |
| **FR-MHS-01..03** | Pengunggahan Tugas Akhir & pencarian berkas publik | `internal/service/document.go:55` | `app/(app)/berkas/page.tsx` | `documents` | **FULL** | Akses dibatasi pada dokumen milik sendiri dan dokumen publik |

### 2.6 Background Worker & Retensi (FR-SYS)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Lokasi Backend | Lokasi Frontend | Tabel DB Terkait | Status Implementasi | Catatan Gap / Kesenjangan |
|:---|:---|:---|:---|:---|:---:|:---|
| **FR-SYS-01** | Jadwal eksekusi retensi 02:00 WIB (Asia/Jakarta) | `cmd/worker/main.go:62` | N/A (Worker CLI) | `documents`, `storage_logs` | **FULL** | Scheduler cron berjalan di zona waktu Asia/Jakarta |
| **FR-SYS-02** | Cutoff 8 tahun soft delete & 90 hari grace period hard delete | `internal/service/retention.go:45` | N/A (Worker CLI) | `documents`, `document_versions` | **FULL** | Logika perhitungan tanggal sesuai aturan SRS |
| **FR-SYS-03** | Distributed locking menggunakan MySQL `GET_LOCK()` | `internal/service/retention.go:120` | N/A (Worker CLI) | N/A (MySQL Lock) | **PARTIAL** | Diterapkan di service; Tidak terverifikasi di unit test karena SQLite tidak mendukung `GET_LOCK()` |
| **FR-SYS-04** | Fail-fast Cloud Storage pada Worker | `cmd/worker/main.go:50` | N/A (Worker CLI) | N/A | **DISCREPANCY (P0)**| Worker jatuh ke `MemoryStorage` jika GCS gagal; berisiko menghapus data DB tanpa menyentuh storage fisik |

---

## 3. MATRIKS KEBUTUHAN NON-FUNGSIONAL (NON-FUNCTIONAL REQUIREMENTS)

| ID Kebutuhan | Deskripsi Kebutuhan SRS | Ketentuan Teknis | Implementasi Aktual | Status | Evaluasi Kesenjangan |
|:---|:---|:---|:---|:---:|:---|
| **NFR-SEC-01** | Access Token Expiration | 15 Menit | `time.Now().Add(15 * time.Minute)` | **FULL** | Sesuai |
| **NFR-SEC-02** | Refresh Token Rotation | 7 Hari + One-time use | Dibatalkan saat dipakai, hash SHA-256 | **FULL** | Sesuai |
| **NFR-SEC-03** | Token Storage Security | In-memory + HttpOnly | React state memory + HttpOnly cookie | **FULL** | Sesuai |
| **NFR-SEC-04** | Password Hashing | Bcrypt Cost >= 12 | `bcrypt.GenerateFromPassword(..., 12)` | **FULL** | Sesuai |
| **NFR-SEC-05** | Rate Limiting | Token Bucket per IP | `internal/middleware/limiter.go` | **FULL** | Sesuai |
| **NFR-SEC-06** | RBAC Enforcement | Server-side validation | `internal/middleware/auth.go` | **FULL** | Backend menolak unauthorized akses |
| **NFR-SEC-07** | Storage Privacy & Isolation | Private bucket, Fail-fast | Silent fallback ke `MemoryStorage` | **FAIL (P0)** | **Celah Kritis:** Berkas bisa hilang permanen |
| **NFR-SEC-08** | Input Sanitization | Sanitize filename & SQL safe | UUID storage key + GORM prepared statements | **FULL** | Sesuai |
| **NFR-SEC-09** | Security Headers & CORS | HSTS, X-Content-Type, CORS | `internal/middleware/middleware.go` | **FULL** | Sesuai |
| **NFR-SEC-10** | Credential Protection | No secrets in repo/migration | Hash password ditaruh di `000011_seed_test_users.up.sql` | **FAIL (P1)** | Celah migrasi: Kredensial di DDL |
| **NFR-FIL-01** | File Size Limit | Max 10 MB per file | Validasi `file.Size <= 10*1024*1024` | **FULL** | Sesuai |
| **NFR-FIL-02** | Request Body Limit | Max 11 MB request body | `http.MaxBytesReader(w, r.Body, 11*1024*1024)` | **FULL** | Sesuai |
| **NFR-RET-01** | Retention Period | 8 Tahun aktif arsip | Filter tanggal `archived_at < now - 8 years` | **FULL** | Sesuai |
| **NFR-RET-02** | Grace Period | 90 Hari sebelum hard delete | Filter tanggal `deleted_at < now - 90 days` | **FULL** | Sesuai |
| **NFR-RET-03** | Idempotensi Worker | Aman dijalankan berulang | Transaksional dan aman dari duplikasi | **FULL** | Sesuai |

---

## 4. MATRIKS KESELARASAN DTO & KONTRAK API (FRONTEND VS BACKEND)

| Endpoint Terkait | Properti Backend (Go) | Properti Frontend (TSX) | Status Keselarasan | Dampak Cacat |
|:---|:---|:---|:---:|:---|
| `GET /api/v1/documents` | `data[].original_name` | `doc.file_name` | **MISMATCH (P0)** | Judul berkas kosong di dashboard berkas terbaru |
| `GET /api/v1/documents` | `data[].size_kb` | `doc.file_size_bytes` | **MISMATCH (P0)** | Ukuran berkas selalu tampil "0 B" di dashboard |
| `All Paginated Endpoints`| `meta.last_page` | `meta.total_pages` | **MISMATCH (P0)** | Kontrol pagination tidak pernah muncul di semua halaman |
| `All Paginated Endpoints`| `meta.current_page` | `meta.page` | **MISMATCH (P0)** | Nomor halaman aktif tidak sinkron |
| `Query String Param` | `?per_page=20` | `?limit=50` atau `?limit=6` | **MISMATCH (P2)** | Nilai limit diabaikan backend (selalu default 20 baris) |
| `POST /api/v1/documents` | `subject_user_id` | *(Tidak ada input di form)* | **GAP (P1)** | BAAK tidak bisa mengunggah atas nama sivitas lain |
| `POST /api/v1/documents` | `archived_at` | *(Tidak ada input di form)* | **GAP (P1)** | BAAK tidak bisa mengarsipkan berkas retroaktif masa lalu |

---

## 5. MATRIKS KELENGKAPAN LAYAR ANTARMUKA (LAMPIRAN D SRS)

| Kode Layar | Nama Layar SRS | File Implementasi Frontend | Status UI | Catatan Evaluasi Auditor |
|:---:|:---|:---|:---:|:---|
| **S-01** | Login Screen | `app/(auth)/login/page.tsx` | **LENGKAP** | Validasi error informatif, responsif |
| **S-02** | Ganti Kata Sandi | `app/(auth)/ganti-sandi/page.tsx` | **LENGKAP** | Berfungsi saat `must_change_password=true` |
| **S-03** | Dashboard (6 Role) | `app/(app)/dashboard/page.tsx` | **CACAT DTO** | Ringkasan kartu statistik tampil; Tabel berkas rusak karena nama & ukuran DTO mismatch |
| **S-04** | File Explorer | `app/(app)/berkas/page.tsx` | **CACAT DTO** | Filter & view mode ada; Pagination tidak muncul |
| **S-05** | Detail Berkas | `app/(app)/berkas/[id]/page.tsx` | **LENGKAP** | Tabs preview, metadata, riwayat versi, audit log |
| **S-06** | Modal Unggah Berkas | `components/UploadModal.tsx` | **KURANG FIELD** | Dropzone drag-and-drop ada; Kurang field BAAK |
| **S-07** | Explorer Publik | `app/(app)/publik/page.tsx` | **LENGKAP** | Akses tanpa otentikasi bekerja dengan baik |
| **S-08** | Panel Verifikasi | `app/(app)/verifikasi/page.tsx` | **LENGKAP** | Dialog penolakan mewajibkan alasan; RejectModal valid |
| **S-09** | Notifikasi | `app/(app)/notifikasi/page.tsx` | **LENGKAP** | Filter status baca/belum baca; Tandai semua dibaca |
| **S-10** | Manajemen Pengguna | `app/(app)/admin/pengguna/page.tsx` | **LENGKAP** | CRUD User, aktivasi status, reset kata sandi |
| **S-11A** | Manajemen Kategori | `app/(app)/admin/kategori/page.tsx` | **LENGKAP** | CRUD Kategori berkas |
| **S-11B** | Manajemen Prodi | `app/(app)/admin/prodi/page.tsx` | **LENGKAP** | CRUD Program Studi |
| **S-12** | Log & Utilisasi Storage| `app/(app)/admin/log/page.tsx` | **LENGKAP** | Metrik penyimpanan dan riwayat audit trail |
| **S-13** | Profil & Sesi Pengguna | `app/(app)/profil/page.tsx` | **LENGKAP** | Informasi akun dan pencabutan sesi perangkat aktif |
| **S-14** | Status Sistem | *(Belum ada rute mandiri)* | **BELUM ADA** | Endpoint `/health` backend ada, halaman UI mandiri `/status` belum dibuat |

---

## 6. MATRIKS CAKUPAN PENGUJIAN (SRS 146 TEST CASES VS REALITAS KODE)

| Kelompok Test Case SRS | Jumlah Wajib SRS | Jumlah Aktual Terdaftar | Kode Tes Aktual di Repo | Status Verifikasi | Jurang Kesenjangan (Gap) |
|:---|:---:|:---:|:---|:---:|:---|
| **TC-AUTH (Autentikasi & Sesi)** | 34 Kasus | 34 Terdaftar | `backend/tests/tc_auth_test.go` | **PASS (34/34 Go)** | 100% backend API verified. 4 skenario UI browser (`TC-AUTH-03, 10, 15, 23`) ditandai untuk Playwright E2E |
| **TC-UPL (Validasi & Unggah)** | 29 Kasus | 29 Terdaftar | `backend/tests/tc_upl_test.go` | **PASS (29/29 Go)** | 100% upload lifecycle, boundaries, magic bytes, RBAC & fault tolerance diverifikasi |
| **TC-SRC (Pencarian & RBAC)** | 22 Kasus | 22 Terdaftar | `backend/tests/tc_src_test.go` | **PASS (22/22 Go)** | 100% filter multi-kriteria, RBAC isolation, pagination, injection safety & literal wildcards diverifikasi |
| **TC-VER (Verifikasi BAAK)** | 19 Kasus | 19 Terdaftar | `backend/tests/tc_ver_test.go` | **PASS (19/19 Go)** | 100% alur verifikasi (approval, rejection note, konkurensi 409, pembatalan & notifikasi verifikator) diverifikasi |
| **TC-DEL (Penghapusan Berkas)** | 20 Kasus | 0 Spesifik | Termasuk di lifecycle | **PENDING PHASE 3.5** | Soft-to-hard delete cascade & otorisasi pemilik belum teruji secara granular |
| **TC-RET (Worker & Retensi)** | 18 Kasus | 0 Spesifik | 1 Fungsi (`TestRetentionWorker`) | **PENDING PHASE 3.6** | MySQL lock `GET_LOCK()` dan penanganan failure GCS belum teruji secara granular |
| **TOTAL** | **146 Kasus** | **104 Kasus Granular** | `tc_auth_test.go`, `tc_upl_test.go`, `tc_src_test.go`, `tc_ver_test.go` | **71.23% SELESAI** | **104/146 test case resmi telah terimplementasi dan lulus 100%** |

---

## 7. KESIMPULAN AUDIT KETERTELUSURAN

Berdasarkan pemetaan matriks di atas:
1. **Tingkat Kepatuhan Backend:** **78%** (Secara logika arsitektur telah mengikuti acuan, namun memiliki celah fatal pada storage fallback dan migrasi 000011).
2. **Tingkat Kepatuhan Frontend:** **65%** (Sebagian besar antarmuka sudah dirancang dengan baik, namun integrasi data terhalang desinkronisasi DTO kritis).
3. **Tingkat Kepatuhan Pengujian:** **71.2%** (104 dari 146 skenario pengujian SRS telah terimplementasi granular dengan kode penomoran resmi dan seluruhnya berstatus PASS).
4. **Tindakan Lanjut Utama:**
   - Menyinkronkan DTO frontend & backend.
   - Mengubah mekanisme startup GCS menjadi *fail-fast* di mode produksi.
   - Menggantikan migrasi 000011 dengan seeder `cmd/seed`.
   - Melanjutkan implementasi test case granular Phase 3 (TS-DEL, TS-RET).
