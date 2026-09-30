<?php

namespace Database\Seeders;

use App\Models\Department;
use App\Models\Role;
use App\Models\User;
use Illuminate\Database\Seeder;
use Illuminate\Support\Facades\Hash;

class UserSeeder extends Seeder
{
    public function run(): void
    {
        $roles = Role::pluck('id', 'slug');
        $deptIF = Department::where('code', 'IF')->first();
        $deptSI = Department::where('code', 'SI')->first();
        $deptTE = Department::where('code', 'TE')->first();

        $users = [
            [
                'name' => 'Super Administrator FITE',
                'email' => 'superadmin@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['superadmin'],
                'department_id' => null,
                'identifier_number' => 'DEV-001',
            ],
            [
                'name' => 'Staf BAAK FITE',
                'email' => 'baak@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['admin_baak'],
                'department_id' => null,
                'identifier_number' => 'ADM-BAAK-01',
            ],
            [
                'name' => 'Prof. Dr. Dekan FITE, M.T.',
                'email' => 'dekan@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['dekan'],
                'department_id' => null,
                'identifier_number' => 'NIP-19750101001',
            ],
            [
                'name' => 'Kaprodi S1 Informatika',
                'email' => 'kaprodi.if@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['kaprodi'],
                'department_id' => $deptIF->id,
                'identifier_number' => 'NIDN-0102030401',
            ],
            [
                'name' => 'Dr. Dosen S1 Sistem Informasi',
                'email' => 'dosen.si@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['dosen'],
                'department_id' => $deptSI->id,
                'identifier_number' => 'NIDN-0102030402',
            ],
            [
                'name' => 'Teaching Assistant Lab Elektro',
                'email' => 'asisten.te@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['staf_ta'],
                'department_id' => $deptTE->id,
                'identifier_number' => 'TA-2026-TE01',
            ],
            [
                'name' => 'Mahasiswa Informatika Demo',
                'email' => 'mahasiswa.if@fite.institution.ac.id',
                'password' => Hash::make('password123'),
                'role_id' => $roles['mahasiswa'],
                'department_id' => $deptIF->id,
                'identifier_number' => '11S22001',
            ],
        ];

        foreach ($users as $userData) {
            User::updateOrCreate(['email' => $userData['email']], $userData);
        }
    }
}