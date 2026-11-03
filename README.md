# SIFITE
## Sistem Informasi Arsip Berkas Digital FITE
 
**Dokumentasi Proyek dan Panduan Penggunaan Teknis**
 
SIFITE merupakan sistem informasi untuk pengelolaan dan pengarsipan berkas digital di Fakultas Informatika dan Teknik Elektro (FITE), Institut Teknologi Del. Sistem ini ditujukan untuk mendukung pengelolaan dokumen pada tiga program studi:
 
- S1 Informatika (IF)
- S1 Sistem Informasi (SI)
- S1 Teknik Elektro (TE)
 
Dokumentasi ini disusun berdasarkan spesifikasi **Software Requirements Specification (SRS) FITE**, Bab 1–6, serta Lampiran D yang memuat mockup antarmuka S-01 sampai dengan S-14.
 
---
 
## Daftar Isi
 
1. [Ringkasan Proyek](#1-ringkasan-proyek)
2. [Status Pengembangan](#2-status-pengembangan)
3. [Gambaran Umum Sistem](#3-gambaran-umum-sistem)
4. [Siklus Hidup Dokumen](#4-siklus-hidup-dokumen)
5. [Matriks Hak Akses](#5-matriks-hak-akses)
6. [Akun Pengujian](#6-akun-pengujian)
7. [Arsitektur Teknologi dan Keamanan](#7-arsitektur-teknologi-dan-keamanan)
8. [Struktur Repositori](#8-struktur-repositori)
9. [Prasyarat Instalasi](#9-prasyarat-instalasi)
10. [Konfigurasi dan Menjalankan Sistem](#10-konfigurasi-dan-menjalankan-sistem)
11. [Pengujian dan Pemeriksaan Kualitas](#11-pengujian-dan-pemeriksaan-kualitas)
12. [Catatan Konsistensi yang Perlu Diverifikasi](#12-catatan-konsistensi-yang-perlu-diverifikasi)
 
---
 
## 1. Ringkasan Proyek
 
SIFITE dikembangkan sebagai sistem terpusat untuk mengelola dan mengarsipkan dokumen akademik maupun operasional FITE. Sistem ini ditujukan untuk mengurangi fragmentasi penyimpanan dokumen serta risiko kehilangan berkas yang tersimpan secara terpisah.
 
Melalui SIFITE, civitas akademika dapat mengunggah, mengelola, menemukan, dan mengakses dokumen sesuai dengan peran serta hak akses masing-masing. Dokumen yang ditetapkan sebagai dokumen publik juga melalui proses verifikasi sebelum dapat diakses oleh pengguna lain.
 
### Tujuan utama
 
- Menyediakan satu tempat pengelolaan arsip digital di lingkungan FITE.
- Mengatur akses dokumen berdasarkan peran pengguna.
- Mendukung proses verifikasi dokumen yang akan dipublikasikan.
- Menyediakan mekanisme pengelolaan masa retensi dan penghapusan dokumen.
- Mendukung pemantauan dokumen melalui dashboard dan fitur administrasi.
 
---
 
## 2. Status Pengembangan
 
Berdasarkan dokumentasi proyek yang diberikan, pengembangan dibagi menjadi tujuh fase. Seluruh fase dicatat berstatus **Selesai**.
 
| Fase | Modul atau ruang lingkup | Cakupan pekerjaan | Status |
|---|---|---|---|
| 1 | Repository Setup & Database Migrations | Struktur monorepo, migrasi SQL yang dicatat pada fase ini sebagai 000001–000010, skema 9 tabel, penggunaan `DATETIME(3)` dalam UTC, serta data referensi awal yang terdiri atas 6 peran, 3 program studi, dan 16 kategori. | Selesai |
| 2 | Go Backend Core & Foundations | Config loader berbasis `.env`, format respons JSON `httpx`, domain errors `apperr`, custom validator `phone` dan `strongpw`, serta middleware CORS, Security Headers, RequestID, dan LimitBody 11 MB. | Selesai |
| 3 | Authentication & Session Management | Login menggunakan bcrypt cost 12 dan perlindungan timing attack melalui `dummyHash`; access token JWT 15 menit yang disimpan di RAM; refresh token 32 byte melalui cookie HttpOnly; rotasi token atomik; deteksi penggunaan ulang token; serta rate limiter lima kegagalan autentikasi dengan respons HTTP 429. | Selesai |
| 4 | Document Management & Verification Engine | Upload multipart dengan pemeriksaan MIME sniffing dan whitelist ekstensi; mekanisme kompensasi untuk kegagalan penyimpanan GCS (SLA-DI-03); pengaturan hak baca melalui `VisibleTo`; antrean verifikasi FIFO untuk dokumen publik dengan status Approved atau Rejected; serta soft delete dan hard delete. | Selesai |
| 5 | Worker & File Retention (Cron) | Background worker harian pukul 02.00 WIB (Asia/Jakarta) menggunakan MySQL distributed lock melalui `GET_LOCK`; soft delete otomatis untuk dokumen dengan masa arsip lebih dari 8 tahun; hard delete setelah masa tenggang 90 hari; serta pembersihan sesi yang tidak aktif lebih dari 30 hari. | Selesai |
| 6 | Next.js Frontend, RBAC Guards & Dashboards | Frontend Next.js 14 App Router; penyimpanan token di memori aplikasi; HTTP interceptor untuk auto-refresh ketika menerima 401; sidebar adaptif berdasarkan peran; antarmuka S-01–S-14; dan modal upload drag-and-drop. | Selesai |
| 7 | Integration Tests, Verification & Final Polish | Pengujian otomatis E2E untuk TS-AUTH, TS-UPL, TS-SRC, TS-VER, TS-DEL, dan TS-RET; pemeriksaan `go vet`; pemeriksaan `npm run lint` dan `tsc --noEmit`; serta perapian dokumentasi. | Selesai |
 
**Catatan:** Status di atas merupakan status yang tercantum dalam dokumen sumber. Status tersebut tidak berarti seluruh hasil pengujian telah diverifikasi ulang dalam penyusunan dokumen ini.
 
---
 
## 3. Gambaran Umum Sistem
 
SIFITE menerapkan pendekatan **single source of truth**, yaitu pengelolaan arsip digital melalui satu sistem terpusat. Penyimpanan berkas dilakukan melalui Google Cloud Storage (GCS), sedangkan informasi atau metadata dokumen dikelola melalui basis data.
 
Sistem menyediakan beberapa kemampuan utama:
 
1. **Pengelolaan dokumen:** pengguna dapat mengunggah dan mengelola dokumen sesuai hak aksesnya.
2. **Pengaturan visibilitas:** dokumen dapat ditetapkan sebagai `Private` atau `Public`.
3. **Verifikasi dokumen publik:** dokumen publik harus melalui proses persetujuan atau penolakan.
4. **Penelusuran dokumen:** pengguna dapat mengakses berkas yang diizinkan melalui File Explorer atau Explorer Publik.
5. **Pengelolaan retensi:** dokumen mengikuti aturan masa penyimpanan dan penghapusan otomatis.
6. **Administrasi sistem:** Super Admin dapat mengelola data pengguna, kategori, program studi, dan log.
 
### 3.1 Jenis dokumen dan batas unggahan
 
Dokumen sumber menyebutkan format berkas yang dapat diunggah meliputi:
 
- PDF
- Dokumen Office: DOCX, XLSX, dan PPTX
- ZIP
- Gambar
 
Ukuran maksimum berkas yang disebutkan adalah **10 MB**. Sistem juga menerapkan pemeriksaan tipe MIME dan ekstensi berkas.
 
---
 
## 4. Siklus Hidup Dokumen
 
Siklus hidup dokumen dimulai ketika pengguna mengunggah berkas dan berakhir ketika dokumen dihapus sesuai kebijakan retensi atau dihapus melalui tindakan yang diizinkan.
 
### 4.1 Pengunggahan dokumen
 
Pengguna mengunggah dokumen melalui mekanisme multipart upload. Pada proses ini, pengguna:
 
1. Memilih berkas yang akan diunggah.
2. Memilih kategori dokumen yang tersedia untuk perannya.
3. Menentukan visibilitas dokumen, yaitu `Private` atau `Public`.
4. Mengirimkan berkas untuk diproses oleh sistem.
 
Format yang disebutkan dalam dokumentasi meliputi PDF, DOCX, XLSX, PPTX, ZIP, dan gambar. Batas ukuran unggahan adalah 10 MB.
 
### 4.2 Penyimpanan dan kompensasi kegagalan
 
Berkas dikirim ke Google Cloud Storage (GCS). Jika penyimpanan metadata ke basis data gagal setelah objek berkas tersimpan di GCS, sistem menjalankan mekanisme kompensasi dengan menghapus objek tersebut.
 
Mekanisme ini mengacu pada ketentuan **SLA-DI-03** dan ditujukan untuk mencegah terbentuknya *orphan files*, yaitu berkas yang tersimpan di cloud tetapi tidak memiliki metadata yang sesuai di basis data.
 
### 4.3 Antrean verifikasi dokumen publik
 
Jika pengguna memilih visibilitas `Public`, dokumen masuk ke status `pending` dan secara otomatis dimasukkan ke antrean verifikasi. Verifikasi dilakukan oleh staf BAAK atau Super Admin sesuai hak akses yang ditetapkan.
 
### 4.4 Persetujuan atau penolakan
 
Verifikator dapat mengambil salah satu keputusan berikut.
 
**a. Disetujui (Approved)**
 
- Status dokumen berubah menjadi `approved`.
- Visibilitas dokumen menjadi `public`.
- Dokumen ditampilkan pada Explorer Publik Civitas.
- Pengunggah menerima notifikasi persetujuan.
 
**b. Ditolak (Rejected)**
 
- Verifikator wajib mengisi alasan penolakan sepanjang 5–500 karakter.
- Visibilitas dokumen dikembalikan menjadi `private`.
- Status dokumen berubah menjadi `rejected`.
- Pengunggah menerima notifikasi yang memuat alasan penolakan.
 
### 4.5 Retensi dan penghapusan dokumen
 
Dokumentasi menetapkan tahapan retensi sebagai berikut:
 
| Tahapan | Ketentuan | Tindakan sistem |
|---|---|---|
| Masa arsip | Dokumen telah berusia lebih dari 8 tahun kalender berdasarkan `archived_at`. | Dokumen menjalani soft delete otomatis oleh background worker. |
| Masa tenggang | Dokumen telah berada dalam status terhapus selama sekurang-kurangnya 90 hari kalender. | Dokumen menjalani hard delete secara permanen dari cloud storage dan basis data. |
 
Selain pengelolaan retensi dokumen, worker juga membersihkan sesi yang berusia lebih dari 30 hari.
 
**Jadwal worker:** setiap hari pukul 02.00 WIB pada zona waktu `Asia/Jakarta`, berdasarkan keterangan pada dokumen sumber.
 
---
 
## 5. Matriks Hak Akses
 
SIFITE menerapkan **Role-Based Access Control (RBAC)** dengan enam peran pengguna. Hak akses setiap peran dibedakan berdasarkan cakupan dashboard, akses dokumen, verifikasi, notifikasi, dan administrasi.
 
| Peran | Dashboard | File Explorer | Explorer Publik | Verifikasi | Pusat Notifikasi | Panel Admin Master Data | Penghapusan permanen (`?force=true`) |
|---|---|---|---|---|---|---|---|
| Super Admin | Semua statistik | Seluruh berkas fakultas | Ya | Setujui atau tolak | Ya | Pengguna, kategori, prodi, dan log | Ya, eksklusif |
| Dekan | Agregat fakultas | Seluruh berkas fakultas | Ya | Tidak | Ya | Tidak | Tidak |
| BAAK | Operasional | Berkas 3 prodi dan operasional | Ya | Setujui atau tolak | Ya | Tidak | Tidak; soft delete operasional |
| Kaprodi | Program studi | Dosen se-fakultas dan mahasiswa prodinya | Ya | Tidak | Ya | Tidak | Tidak |
| Dosen | Dosen | Mahasiswa prodinya dan berkas pribadi | Ya | Tidak | Ya | Tidak | Tidak |
| Mahasiswa | Mahasiswa | Berkas pribadi sendiri | Ya | Tidak | Ya | Tidak | Tidak |
 
**Keterangan:** Matriks ini mempertahankan cakupan hak akses sebagaimana ditulis pada dokumen sumber. Rincian kebijakan akses pada tingkat endpoint atau kondisi khusus perlu merujuk pada implementasi dan SRS.
 
---
 
## 6. Akun Pengujian
 
Bagian ini memuat akun yang dicantumkan dalam dokumentasi untuk menguji alur kerja dan variasi hak akses antarpengguna.
 
**Password default yang tercantum pada dokumen sumber:** `Password123!`
 
> **Peringatan keamanan:** kredensial berikut hanya boleh digunakan pada lingkungan pengujian yang terisolasi. Jangan menggunakan password default ini pada lingkungan produksi. Ganti seluruh password sebelum sistem dipublikasikan atau digunakan dengan data nyata. Jangan menyimpan kredensial produksi di repositori atau dokumentasi publik.
 
| No. | Peran | Email | Nama pengguna | Program studi | Skenario pengujian |
|---:|---|---|---|---|---|
| 1 | Mahasiswa | `iss24034@del.ac.id` | Mahasiswa Sistem Informasi | S1 SI (NIM: `iss24034`) | Mengunggah berkas skripsi privat dan publik serta mengubah visibilitas. |
| 2 | Mahasiswa | `budi@del.ac.id` | Budi Mahasiswa (IF) | S1 IF (NIM: `11S20001`) | Menguji isolasi data: akun Budi tidak dapat melihat berkas privat mahasiswa SI. |
| 3 | BAAK | `baak@del.ac.id` | Staf BAAK | — | Membuka menu Verifikasi serta menyetujui atau menolak berkas publik yang diunggah mahasiswa. |
| 4 | Dosen | `dosen.if@del.ac.id` | Dosen Informatika | S1 IF | Melihat berkas mahasiswa bimbingan IF serta mengunggah RPS dan SK Mengajar. |
| 5 | Kaprodi | `kaprodi.if@del.ac.id` | Kaprodi Informatika | S1 IF | Memantau berkas dosen se-fakultas dan mahasiswa program studi IF. |
| 6 | Dekan | `dekan@del.ac.id` | Dekan FITE | — | Memantau dashboard ringkasan fakultas dan arsip seluruh program studi. |
| 7 | Super Admin | `admin@del.ac.id` | Super Admin FITE | — | Mengakses seluruh fitur serta mengelola pengguna, kategori, program studi, dan audit log penyimpanan. |
 
---
 
## 7. Arsitektur Teknologi dan Keamanan
 
### 7.1 Komponen teknologi
 
Berdasarkan struktur dan uraian pada dokumen sumber, teknologi utama yang digunakan adalah:
 
| Komponen | Teknologi atau mekanisme | Peran |
|---|---|---|
| Backend | Go | Menyediakan layanan dan REST API. |
| Frontend | Next.js 14 App Router | Menyediakan antarmuka pengguna. |
| Basis data | MySQL 8 | Menyimpan data aplikasi dan metadata dokumen. |
| Penyimpanan berkas | Google Cloud Storage (GCS) | Menyimpan objek atau berkas yang diunggah. |
| Autentikasi | JWT, bcrypt, dan refresh token | Mengelola autentikasi serta sesi pengguna. |
| Background worker | Cron dan MySQL `GET_LOCK` | Menjalankan proses retensi dan pembersihan secara terjadwal. |
 
### 7.2 Perlindungan keamanan
 
**a. Perlindungan terhadap XSS (NFR-SEC-04)**
 
Access token JWT hanya disimpan di RAM atau memori aplikasi. Token tidak disimpan di `localStorage` maupun `sessionStorage`.
 
**b. Pengelolaan refresh token dan cookie**
 
Refresh token disimpan melalui cookie browser dengan atribut:
 
- `HttpOnly`
- `SameSite=Strict`
- `Secure` pada lingkungan produksi
- `Path=/api/v1/auth`
 
**c. Deteksi penggunaan ulang token**
 
Refresh token dirotasi pada setiap permintaan. Jika token lama digunakan kembali, sistem menganggap sesi berpotensi telah dicuri dan mencabut seluruh sesi akun terkait.
 
**d. Perlindungan brute force**
 
Rate limiter membatasi lima kegagalan autentikasi berturut-turut sebelum menerapkan pembatasan dengan respons HTTP `429 Too Many Requests`.
 
**e. Perlindungan unggahan berkas**
 
Sistem menerapkan:
 
- Batas ukuran berkas 10 MB dengan respons HTTP `422`.
- Batas maksimum request body 11 MB dengan respons HTTP `413`.
- Pemeriksaan MIME type melalui MIME sniffing.
- Whitelist ekstensi berkas.
- Pencegahan ekstensi ganda yang berpotensi berbahaya, misalnya `laporan.pdf.exe`.
 
---
 
## 8. Struktur Repositori
 
Proyek menggunakan struktur **monorepo** yang memisahkan backend, frontend, dokumentasi, dan berkas pendukung.
 
```text
.
├── backend/                         # Layanan backend Go
│   ├── cmd/
│   │   ├── api/                     # Entrypoint HTTP REST API server
│   │   ├── seed/                    # Seeder akun pengujian otomatis
│   │   └── worker/                  # Entrypoint background retention worker
│   ├── internal/
│   │   ├── app/                     # Inisialisasi database dan koneksi pool
│   │   ├── apperr/                  # Katalog galat domain dan mapping HTTP status
│   │   ├── config/                  # Pemuatan dan validasi konfigurasi environment
│   │   ├── domain/                  # Model domain GORM dan tipe khusus JSON/List
│   │   ├── handler/                 # Request handler (Auth, Doc, Verif, User, dll.)
│   │   ├── httpx/                   # Envelope JSON, validator, dan binding helper
│   │   ├── middleware/              # Auth Bearer, CORS, CSP, LimitBody, RateLimiter
│   │   ├── router/                  # Pendaftaran endpoint dan guard middleware
│   │   ├── service/                 # Logika bisnis aplikasi
│   │   └── storage/                 # Abstraksi GCS dan mock storage memori
│   ├── migrations/                 # Migrasi skema SQL
│   └── tests/                      # Suite integrasi otomatis dan pengujian E2E
├── frontend/                        # Klien web Next.js 14 App Router
│   ├── app/                         # Struktur App Router (S-01 sampai S-14)
│   ├── components/                  # Komponen UI responsif dan modal interaktif
│   └── lib/                         # API client, interceptor, dan auth state
├── docs/                            # Dokumen SRS FITE dan spesifikasi teknis
└── README.md
```
 
---
 
## 9. Prasyarat Instalasi
 
Sebelum menjalankan sistem, pastikan perangkat lunak berikut tersedia.
 
| Perangkat lunak | Versi minimum atau keterangan |
|---|---|
| Go | 1.22 atau lebih baru |
| Node.js | 18 atau lebih baru |
| npm | Disertakan atau tersedia bersama instalasi Node.js |
| MySQL | 8.0.16 atau lebih baru |
| `golang-migrate` CLI | Opsional menurut dokumen sumber; diperlukan jika migrasi dijalankan menggunakan CLI `migrate`. |
 
Pastikan layanan MySQL telah aktif dan database yang digunakan sudah disiapkan sebelum menjalankan backend.
 
---
 
## 10. Konfigurasi dan Menjalankan Sistem
 
Langkah berikut merangkum perintah yang tercantum pada dokumentasi sumber. Sesuaikan nilai konfigurasi dengan lingkungan lokal sebelum menjalankannya.
 
### 10.1 Menyiapkan konfigurasi backend
 
Masuk ke direktori backend:
 
```bash
cd backend
```
 
Salin berkas konfigurasi contoh:
 
```bash
cp .env.example .env
```
 
Buka berkas `.env`, kemudian sesuaikan konfigurasi koneksi database. Contoh nilai DSN yang tercantum pada dokumen sumber:
 
```env
DB_DSN=fite_app:rahasia@tcp(127.0.0.1:3306)/fite_arsip?charset=utf8mb4&parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27
```
 
Ganti nama pengguna, password, host, port, dan nama database sesuai konfigurasi MySQL yang digunakan. Jangan memakai kredensial contoh untuk lingkungan produksi.
 
### 10.2 Menjalankan migrasi database
 
Dokumentasi sumber mencantumkan perintah berikut:
 
```bash
migrate -path migrations -database "mysql://$DB_DSN" up
```
 
Periksa kembali format URL koneksi yang diterima oleh versi `golang-migrate` dan driver MySQL yang digunakan. Nilai `DB_DSN` pada contoh di atas berbentuk DSN driver Go; format tersebut belum tentu dapat langsung digunakan sebagai URL koneksi `golang-migrate`. Gunakan format koneksi yang sesuai dengan dokumentasi versi tool yang terpasang sebelum menjalankan migrasi.
 
### 10.3 Mengisi data awal
 
Jalankan seeder dari direktori `backend`:
 
```bash
go run ./cmd/seed
```
 
Menurut dokumentasi sumber, perintah tersebut mendaftarkan tujuh akun pengujian yang tercantum pada bagian [Akun Pengujian](#6-akun-pengujian) ke database MySQL.
 
### 10.4 Menjalankan backend API
 
Buka terminal baru, kemudian jalankan:
 
```bash
cd backend
go run ./cmd/api
```
 
Dokumentasi sumber menyebutkan bahwa server REST API berjalan pada:
 
```text
http://localhost:8080
```
 
### 10.5 Menjalankan background worker
 
Worker dapat dijalankan dalam dua mode.
 
**Mode sekali jalan**
 
Mode ini ditujukan untuk pengujian atau pemeriksaan proses secara langsung:
 
```bash
cd backend
go run ./cmd/worker -once
```
 
**Mode daemon terjadwal**
 
Mode ini menjalankan worker sebagai proses terjadwal dengan waktu eksekusi harian pukul 02.00 WIB:
 
```bash
cd backend
go run ./cmd/worker
```
 
Pastikan konfigurasi zona waktu dan mekanisme penjadwalan worker sesuai dengan lingkungan tempat aplikasi dijalankan.
 
### 10.6 Menjalankan frontend
 
Buka terminal baru dan masuk ke direktori frontend:
 
```bash
cd frontend
```
 
Pasang dependensi:
 
```bash
npm install
```
 
Jalankan server pengembangan:
 
```bash
npm run dev
```
 
Aplikasi frontend dapat diakses melalui browser pada alamat:
 
```text
http://localhost:3000
```
 
### 10.7 Urutan menjalankan sistem
 
Secara umum, urutan kerja untuk menjalankan aplikasi secara lokal adalah:
 
1. Pastikan MySQL aktif dan database telah disiapkan.
2. Siapkan konfigurasi backend melalui `.env`.
3. Jalankan migrasi database.
4. Jalankan seeder untuk mengisi akun pengujian.
5. Jalankan backend API.
6. Jalankan background worker jika diperlukan untuk menguji proses retensi.
7. Jalankan frontend.
8. Buka `http://localhost:3000` melalui browser.
 
---
 
## 11. Pengujian dan Pemeriksaan Kualitas
 
Dokumentasi sumber mencantumkan pengujian backend, pengujian integrasi, pemeriksaan kualitas frontend, dan build produksi.
 
### 11.1 Pengujian backend
 
Masuk ke direktori backend:
 
```bash
cd backend
```
 
Jalankan seluruh unit test dan pengujian integrasi:
 
```bash
go test -v ./...
```
 
Jalankan pengujian spesifikasi SRS:
 
```bash
go test -v ./tests
```
 
Kelompok test case yang disebutkan meliputi:
 
| Kode pengujian | Ruang lingkup yang tercantum |
|---|---|
| TS-AUTH | Autentikasi |
| TS-UPL | Upload dokumen |
| TS-SRC | Tidak dirinci lebih lanjut dalam dokumen sumber |
| TS-VER | Verifikasi dokumen |
| TS-DEL | Penghapusan dokumen |
| TS-RET | Retensi dokumen |
 
### 11.2 Pemeriksaan kualitas frontend
 
Masuk ke direktori frontend:
 
```bash
cd frontend
```
 
**Pemeriksaan tipe TypeScript**
 
```bash
npx tsc --noEmit
```
 
**Pemeriksaan linting ESLint**
 
```bash
npm run lint
```
 
**Build produksi Next.js**
 
```bash
npm run build
```
 
### 11.3 Status hasil pemeriksaan yang dicatat
 
Dokumentasi sumber menyatakan bahwa perintah pemeriksaan tersebut telah dijalankan dengan hasil **0 error dan 0 warning**.
 
Pernyataan ini dipertahankan sebagai catatan dari dokumen sumber dan belum diuji ulang pada saat penyuntingan dokumentasi ini.
 
---
 
## 12. Catatan Konsistensi yang Perlu Diverifikasi
 
Beberapa informasi pada dokumen sumber perlu diperiksa terhadap repositori, file konfigurasi, SRS, dan implementasi aktual. Catatan ini sengaja dipisahkan agar ketidaksesuaian tidak diperbaiki dengan asumsi.
 
| No. | Bagian | Informasi yang ditemukan | Tindakan verifikasi |
|---:|---|---|---|
| 1 | Jumlah migrasi database | Ringkasan Phase 1 menyebut migrasi `000001`–`000010`, sedangkan struktur repositori menyebut `000001`–`000011`. | Periksa nama berkas di direktori `backend/migrations` dan sesuaikan kedua keterangan berdasarkan jumlah aktual. |
| 2 | Koneksi migrasi | Contoh perintah menggunakan `mysql://$DB_DSN`, sedangkan nilai `DB_DSN` ditulis dalam format DSN Go. | Pastikan format koneksi sesuai dengan versi `golang-migrate` dan driver yang digunakan. |
| 3 | Jadwal worker | Worker disebut berjalan pukul 02.00 WIB dengan zona `Asia/Jakarta`, sedangkan penyimpanan waktu database disebut menggunakan UTC. | Periksa konfigurasi scheduler, konversi zona waktu, dan cara penyimpanan timestamp. |
| 4 | Klaim hasil pengujian | Dokumen menyatakan seluruh pengujian berjalan dengan 0 error dan 0 warning. | Simpan keluaran pengujian terbaru sebagai bukti dan pastikan klaim sesuai dengan kondisi repositori saat ini. |
| 5 | Akun pengujian | Terdapat tujuh akun pengujian dan satu password default yang sama. | Pastikan akun hanya digunakan di lingkungan pengujian serta password diganti sebelum deployment produksi. |
| 6 | Ruang lingkup TS-SRC | Kode test case TS-SRC dicantumkan, tetapi kepanjangannya tidak dijelaskan. | Cocokkan dengan Bab 3 SRS atau definisi test case pada kode pengujian. |
| 7 | Cakupan fase dan struktur repositori | Ringkasan Phase 1 menyebut migrasi sampai `000010`, sedangkan bagian struktur menyebut sampai `000011`; ringkasan juga menyebut skema 9 tabel. | Verifikasi jumlah migrasi dan tabel terhadap skema database aktual. |
 
### Penutup
 
SIFITE dirancang sebagai sistem pengelolaan arsip digital terpusat untuk lingkungan FITE. Fitur yang dijelaskan dalam dokumentasi mencakup pengelolaan dokumen, pengaturan visibilitas, verifikasi dokumen publik, pembatasan akses berdasarkan peran, pengelolaan retensi, serta administrasi sistem.
 
Dokumentasi ini menjadi panduan ringkas mengenai ruang lingkup, struktur teknis, keamanan, proses instalasi, dan pengujian SIFITE. Untuk memastikan kesesuaian implementasi, seluruh konfigurasi dan catatan yang ditandai pada bagian [Catatan Konsistensi yang Perlu Diverifikasi](#12-catatan-konsistensi-yang-perlu-diverifikasi) perlu dicocokkan dengan repositori dan SRS versi yang berlaku.