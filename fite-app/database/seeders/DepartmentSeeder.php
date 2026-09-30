<?php

namespace Database\Seeders;

use App\Models\Department;
use Illuminate\Database\Seeder;
use Illuminate\Support\Str;

class DepartmentSeeder extends Seeder
{
    public function run(): void
    {
        $departments = [
            ['name' => 'S1 Informatika', 'code' => 'IF'],
            ['name' => 'S1 Sistem Informasi', 'code' => 'SI'],
            ['name' => 'D3/D4 Teknik Elektro', 'code' => 'TE'],
        ];

        foreach ($departments as $dept) {
            Department::updateOrCreate(
                ['code' => $dept['code']],
                ['name' => $dept['name'], 'slug' => Str::slug($dept['name'])]
            );
        }
    }
}