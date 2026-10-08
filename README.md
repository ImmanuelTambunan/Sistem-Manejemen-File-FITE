# Sistem Informasi Arsip Berkas Digital FITE

Platform repositori digital terintegrasi Fakultas Informatika dan Teknik Elektro (FITE) berbasis RESTful API dengan backend Golang dan frontend Next.js.

## 🛠️ Tech Stack & Spesifikasi
- **Backend:** Go (v1.22+) dengan framework Gin & GORM
- **Frontend:** Next.js (App Router), React, TypeScript, Tailwind CSS
- **Database:** MySQL 8.x (Engine InnoDB, Collation utf8mb4_unicode_ci)
- **Object Storage:** Google Cloud Storage (GCS)
- **Database Migration:** golang-migrate
- **Otentikasi:** JWT Access Token (Bearer, 15 Menit) + Refresh Token (httpOnly Cookie, 7 Hari)

---

## 👥 Struktur Tim & Tanggung Jawab
- **Dev 1 (Team Lead):** Database Migration, Core Backend Foundation, Auth & Session Engine
- **Dev 2:** Document Management API, GCS Storage Handler, Verification & Retention Worker
- **Dev 3:** Next.js Foundation, Auth Context & Memory Token, App Shell & Dashboard UI
- **Dev 4:** Document Explorer UI, Upload Modal, Verification Panel, Public Repository UI

---

## 🚀 Panduan Menjalankan Proyek di Lokal

### 1. Prasyarat Sistem
- Go Compiler (v1.22 atau lebih baru)
- Node.js (LTS v20+) & npm
- MySQL Server 8.x (berjalan di Laragon atau native service port 3306)
- Git CLI

### 2. Konfigurasi Database (MySQL)
Buat database baru di MySQL CLI atau HeidiSQL:
```sql
CREATE DATABASE fite_arsip CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;