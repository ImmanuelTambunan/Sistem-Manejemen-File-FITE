<?php

namespace Database\Seeders;

use App\Models\Role;
use Illuminate\Database\Seeder;

class RoleSeeder extends Seeder
{
    public function run(): void
    {
        $roles = [
            ['name' => 'SuperAdmin', 'slug' => 'superadmin', 'hierarchy_level' => 1, 'description' => 'Akses penuh konfigurasi web repositori FITE'],
            ['name' => 'Admin BAAK', 'slug' => 'admin_baak', 'hierarchy_level' => 2, 'description' => 'Verifikator dokumen dan akun akademik fakultas'],
            ['name' => 'Dekan', 'slug' => 'dekan', 'hierarchy_level' => 3, 'description' => 'Akses eksekutif dan audit dokumen fakultas'],
            ['name' => 'Kaprodi', 'slug' => 'kaprodi', 'hierarchy_level' => 4, 'description' => 'Validasi repositori tingkat program studi'],
            ['name' => 'Dosen', 'slug' => 'dosen', 'hierarchy_level' => 5, 'description' => 'Pengunggah materi riset dan pengesah berkas TA'],
            ['name' => 'Staf / Teaching Assistant', 'slug' => 'staf_ta', 'hierarchy_level' => 6, 'description' => 'Pengelola modul praktikum dan bahan ajar'],
            ['name' => 'Mahasiswa', 'slug' => 'mahasiswa', 'hierarchy_level' => 7, 'description' => 'Akses unduh dokumen publik dan repositori internal'],
        ];

        foreach ($roles as $role) {
            Role::updateOrCreate(['slug' => $role['slug']], $role);
        }
    }
}