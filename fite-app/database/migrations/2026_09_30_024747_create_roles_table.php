<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('roles', function (Blueprint $table) {
            $table->id();
            $table->string('name'); 
            $table->string('slug', 30)->unique(); // superadmin, admin_baak, dekan, kaprodi, dosen, staf_ta, mahasiswa
            $table->unsignedTinyInteger('hierarchy_level'); // 1: SuperAdmin ... 7: Mahasiswa
            $table->string('description')->nullable();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('roles');
    }
};