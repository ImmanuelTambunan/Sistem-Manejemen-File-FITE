# SIFITE Frontend - Sistem Informasi Arsip Berkas Digital FITE

Frontend aplikasi web Sistem Informasi Pengelolaan dan Pengarsipan Berkas Digital Fakultas Informatika dan Teknik Elektro (FITE) Institut Teknologi Del, dibangun menggunakan **Next.js 14 (App Router)**, **TypeScript**, dan **Tailwind CSS**.

---

## Fitur Utama

- **In-Memory JWT Token Management (NFR-SEC-04):** Access token disimpan murni di memori (React Context) untuk mencegah serangan XSS. Refresh token dikelola otomatis melalui cookie `HttpOnly; SameSite=Strict`.
- **Automatic Token Refresh Interceptor:** Pembaruan token otomatis saat respons HTTP 401 dan pengulangan request asli secara transparan tanpa logout prematur.
- **Role-Based Access Control (RBAC):** Pelindung rute dan navigasi adaptif untuk 6 peran pengguna:
  - Super Admin
  - Dekan
  - BAAK
  - Koordinator Program Studi (Kaprodi)
  - Dosen
  - Mahasiswa
- **Modul Antarmuka (Mockup S-01 s/d S-14):**
  - **S-01 Login & S-02 Ganti Sandi:** Otentikasi dan penegakan penggantian sandi wajib.
  - **S-03 Dashboard Adaptif:** Statistik metrik arsip, tren berkas per kategori, dan distribusi per program studi.
  - **S-04 File Explorer & S-06 Upload:** Penelusuran arsip dengan filter multi-kriteria dan modal unggah drag-and-drop.
  - **S-05 Detail Berkas:** Metadata lengkap, streaming unduhan langsung dari GCS, riwayat verifikasi, dan kontrol hapus berkas.
  - **S-07 Explorer Publik:** Pencarian dan pengunduhan berkas publik civitas akademika berstatus disetujui.
  - **S-08 Verifikasi Berkas:** Antrean verifikasi berkas publik khusus BAAK dan Super Admin.
  - **S-09 Notifikasi:** Notifikasi real-time, badge jumlah pesan belum dibaca, dan dropdown cepat.
  - **S-10 s/d S-12 Panel Admin:** Manajemen data pengguna, kategori berkas, program studi, dan log aktivitas penyimpanan.
  - **S-13 Profil Pengguna:** Pembaruan informasi personal dan pergantian kata sandi berkala.
  - **S-14 Status Pages:** Halaman error dan 404 terstandarisasi.

---

## Prasyarat & Menjalankan Aplikasi

```bash
# Instal dependensi
npm install

# Menjalankan server pengembangan
npm run dev

# Membangun versi produksi
npm run build

# Menjalankan versi produksi
npm run start
```

## Konfigurasi Lingkungan

Konfigurasi dibaca melalui file `.env.local` pada direktori root frontend:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```
