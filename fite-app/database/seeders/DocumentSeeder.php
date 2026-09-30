<?php

namespace Database\Seeders;

use App\Models\Category;
use App\Models\Department;
use App\Models\Document;
use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Str;

class DocumentSeeder extends Seeder
{
    public function run(): void
    {
        $dosen = User::where('email', 'dosen.si@fite.institution.ac.id')->first();
        $mhs = User::where('email', 'mahasiswa.if@fite.institution.ac.id')->first();
        $baak = User::where('email', 'baak@fite.institution.ac.id')->first();

        $deptIF = Department::where('code', 'IF')->first();
        $deptSI = Department::where('code', 'SI')->first();

        $catTA = Category::where('slug', 'source-code-ta')->first();
        $catModul = Category::where('slug', 'mata-kuliah')->first();
        $catInternal = Category::where('slug', 'dokumen-internal-fite')->first();

        $documents = [
            [
                'title' => 'Repository Source Code TA: Sistem Rekomendasi Pemilihan Peminatan dengan Hybrid Filtering',
                'description' => 'Source code lengkap berbasis Laravel & Python REST API untuk tugas akhir mahasiswa S1 Informatika.',
                'author_name' => 'Mahasiswa Informatika Demo',
                'publication_year' => 2026,
                'file_path' => 'documents/public/sample_ta_if.zip',
                'file_extension' => 'zip',
                'file_size' => 18450000,
                'department_id' => $deptIF->id,
                'category_id' => $catTA->id,
                'uploaded_by' => $mhs->id,
                'access_type' => 'public',
                'min_download_hierarchy' => 7,
                'download_count' => 143,
            ],
            [
                'title' => 'Modul Ajar Praktikum: Arsitektur Perangkat Lunak Terdistribusi Semester Genap 2026',
                'description' => 'Panduan praktikum resmi mencakup Microservices, Docker containerization, dan message broker.',
                'author_name' => 'Tim Pengajar S1 Sistem Informasi',
                'publication_year' => 2026,
                'file_path' => 'documents/public/modul_arsitektur_pl.pdf',
                'file_extension' => 'pdf',
                'file_size' => 5200000,
                'department_id' => $deptSI->id,
                'category_id' => $catModul->id,
                'uploaded_by' => $dosen->id,
                'access_type' => 'public',
                'min_download_hierarchy' => 7,
                'download_count' => 89,
            ],
            [
                'title' => 'Laporan Akreditasi & Notulensi Rapat Senat FITE 2026',
                'description' => 'Dokumen rahasia fakultas berisi hasil evaluasi akademik dan audit kinerja internal.',
                'author_name' => 'BAAK FITE & Senat Fakultas',
                'publication_year' => 2026,
                'file_path' => 'documents/private/notulensi_rapat_senat_2026.pdf',
                'file_extension' => 'pdf',
                'file_size' => 2100000,
                'department_id' => $deptIF->id,
                'category_id' => $catInternal->id,
                'uploaded_by' => $baak->id,
                'access_type' => 'private',
                'min_download_hierarchy' => 4,
                'download_count' => 7,
            ],
        ];

        foreach ($documents as $doc) {
            Document::updateOrCreate(
                ['slug' => Str::slug($doc['title'])],
                $doc
            );
        }
    }
}