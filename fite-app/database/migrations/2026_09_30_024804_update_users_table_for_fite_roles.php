<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('users', function (Blueprint $table) {
            $table->foreignId('role_id')->after('id')->constrained('roles')->cascadeOnDelete();
            $table->foreignId('department_id')->nullable()->after('role_id')->constrained('departments')->nullOnDelete();
            $table->string('identifier_number', 30)->nullable()->unique()->after('department_id'); // NIM / NIDN / NIP
            $table->boolean('is_active')->default(true)->after('password');
        });
    }

    public function down(): void
    {
        Schema::table('users', function (Blueprint $table) {
            $table->dropForeign(['role_id']);
            $table->dropForeign(['department_id']);
            $table->dropColumn(['role_id', 'department_id', 'identifier_number', 'is_active']);
        });
    }
};