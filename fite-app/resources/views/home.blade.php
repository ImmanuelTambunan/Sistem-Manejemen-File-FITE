@extends('layouts.repository')

@section('title', 'Beranda - Repositori Digital FITE')

@section('content')
<!-- Hero Section with Deep Academic Navy Theme -->
<div class="relative bg-slate-900 bg-gradient-to-b from-slate-900 via-blue-950 to-slate-900 text-white pt-16 pb-28 px-4 sm:px-6 lg:px-8 overflow-hidden">
    <div class="absolute inset-0 opacity-10 bg-[radial-gradient(#3b82f6_1px,transparent_1px)] [background-size:16px_16px]"></div>
    
    <div class="relative max-w-5xl mx-auto text-center">
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold bg-blue-500/10 text-blue-400 border border-blue-500/20 backdrop-blur-sm mb-4">
            Portal Arsip & Repositori Resmi
        </span>
        <h1 class="text-3xl sm:text-5xl font-black tracking-tight text-white leading-tight">
            Eksplorasi Dokumen, Riset, & <br class="hidden sm:inline"> Source Code Tugas Akhir FITE
        </h1>
        <p class="mt-4 text-base sm:text-lg text-slate-300 max-w-2xl mx-auto leading-relaxed">
            Akses ribuan materi perkuliahan, modul praktikum, paper ilmiah, dan repositori program dari seluruh civitas akademika FITE.
        </p>
    </div>
</div>

<!-- Floating Multi-Category Search Widget (Traveloka Style Box) -->
<div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 -mt-16 relative z-20">
    <div class="bg-white rounded-2xl shadow-xl shadow-slate-900/10 border border-slate-200/80 p-4 sm:p-6 backdrop-blur">
        <form action="{{ route('home') }}" method="GET" class="grid grid-cols-1 md:grid-cols-12 gap-4 items-end">
            <!-- Keyword Search -->
            <div class="md:col-span-4">
                <label class="block text-xs font-bold uppercase tracking-wider text-slate-500 mb-1.5">Kata Kunci / Judul</label>
                <div class="relative">
                    <span class="absolute inset-y-0 left-0 flex items-center pl-3.5 pointer-events-none text-slate-400">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                        </svg>
                    </span>
                    <input type="text" name="search" value="{{ request('search') }}" 
                           placeholder="Cari judul TA, modul, penulis..." 
                           class="w-full pl-10 pr-4 py-2.5 rounded-xl border border-slate-300 text-sm focus:ring-2 focus:ring-blue-600 focus:border-blue-600 placeholder-slate-400">
                </div>
            </div>

            <!-- Program Studi Filter -->
            <div class="md:col-span-3">
                <label class="block text-xs font-bold uppercase tracking-wider text-slate-500 mb-1.5">Program Studi</label>
                <select name="department" class="w-full py-2.5 px-3 rounded-xl border border-slate-300 text-sm focus:ring-2 focus:ring-blue-600 focus:border-blue-600 bg-white">
                    <option value="">Semua Program Studi</option>
                    @foreach($departments as $dept)
                        <option value="{{ $dept->code }}" {{ request('department') == $dept->code ? 'selected' : '' }}>
                            {{ $dept->name }}
                        </option>
                    @endforeach
                </select>
            </div>

            <!-- Tahun Filter -->
            <div class="md:col-span-2">
                <label class="block text-xs font-bold uppercase tracking-wider text-slate-500 mb-1.5">Tahun</label>
                <select name="year" class="w-full py-2.5 px-3 rounded-xl border border-slate-300 text-sm focus:ring-2 focus:ring-blue-600 focus:border-blue-600 bg-white">
                    <option value="">Semua</option>
                    @for($y = date('Y'); $y >= 2020; $y--)
                        <option value="{{ $y }}" {{ request('year') == $y ? 'selected' : '' }}>{{ $y }}</option>
                    @endfor
                </select>
            </div>

            <!-- Submit Button -->
            <div class="md:col-span-3">
                <button type="submit" class="w-full flex items-center justify-center gap-2 py-2.5 px-5 bg-blue-700 hover:bg-blue-800 text-white font-bold rounded-xl shadow-md shadow-blue-700/20 transition-all hover:shadow-lg">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                    </svg>
                    Cari Repositori
                </button>
            </div>
        </form>
    </div>
</div>

<!-- Main Repository Content Grid -->
<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 mt-12">
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-6 border-b border-slate-200">
        <div>
            <h2 class="text-xl font-extrabold text-slate-900 tracking-tight">Koleksi Dokumen Terbaru</h2>
            <p class="text-sm text-slate-500 mt-1">Menampilkan publikasi akademik dan berkas tugas akhir terverifikasi</p>
        </div>
        <div class="mt-4 sm:mt-0 text-xs font-semibold text-slate-500">
            Total {{ $documents->total() }} Dokumen Ditemukan
        </div>
    </div>

    <!-- Cards Grid -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
        @forelse($documents as $doc)
            <div class="bg-white rounded-2xl border border-slate-200/80 shadow-sm hover:shadow-lg transition-all duration-300 flex flex-col justify-between overflow-hidden group">
                <div class="p-6">
                    <!-- Top Card Meta: Department Badge & Access Status -->
                    <div class="flex items-center justify-between gap-2 mb-3">
                        <span class="px-2.5 py-1 rounded-md text-[11px] font-bold tracking-wide uppercase 
                            {{ $doc->department->code == 'IF' ? 'bg-indigo-50 text-indigo-700 border border-indigo-200/60' : '' }}
                            {{ $doc->department->code == 'SI' ? 'bg-emerald-50 text-emerald-700 border border-emerald-200/60' : '' }}
                            {{ $doc->department->code == 'TE' ? 'bg-amber-50 text-amber-700 border border-amber-200/60' : '' }}">
                            {{ $doc->department->name }}
                        </span>

                        @if($doc->isPrivate())
                            <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-bold bg-rose-50 text-rose-700 border border-rose-200/60">
                                <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clip-rule="evenodd"></path>
                                </svg>
                                Private
                            </span>
                        @else
                            <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-bold bg-green-50 text-green-700 border border-green-200/60">
                                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z"></path>
                                </svg>
                                Public
                            </span>
                        @endif
                    </div>

                    <!-- Category Pill -->
                    <span class="text-xs font-semibold text-blue-700 block mb-1.5">{{ $doc->category->name }}</span>

                    <!-- Document Title -->
                    <h3 class="text-base font-bold text-slate-900 group-hover:text-blue-700 transition-colors line-clamp-2 leading-snug">
                        {{ $doc->title }}
                    </h3>

                    <!-- Document Description Snippet -->
                    <p class="mt-2.5 text-xs text-slate-600 line-clamp-2 leading-relaxed">
                        {{ $doc->description ?? 'Tidak ada deskripsi berkas.' }}
                    </p>

                    <!-- Author & Year Meta -->
                    <div class="mt-4 pt-4 border-t border-slate-100 flex items-center justify-between text-xs text-slate-500">
                        <span class="truncate max-w-[170px] font-medium text-slate-700" title="{{ $doc->author_name }}">
                            👤 {{ $doc->author_name }}
                        </span>
                        <span class="font-medium">Tahun {{ $doc->publication_year }}</span>
                    </div>
                </div>

                <!-- Card Bottom Bar (File Type, Size, & Action) -->
                <div class="px-6 py-3.5 bg-slate-50/80 border-t border-slate-100 flex items-center justify-between">
                    <div class="flex items-center gap-2">
                        <span class="uppercase font-extrabold text-[10px] px-2 py-0.5 rounded bg-slate-200 text-slate-700">
                            {{ $doc->file_extension }}
                        </span>
                        <span class="text-xs text-slate-500">
                            {{ number_format($doc->file_size / (1024 * 1024), 2) }} MB
                        </span>
                    </div>

                    <div class="flex items-center gap-3">
                        <span class="flex items-center gap-1 text-xs font-medium text-slate-500">
                            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path>
                            </svg>
                            {{ $doc->download_count }}
                        </span>
                        <a href="#" class="text-xs font-bold text-blue-700 hover:text-blue-900 transition-colors">
                            Detail &rarr;
                        </a>
                    </div>
                </div>
            </div>
        @empty
            <div class="col-span-full py-16 text-center bg-white rounded-2xl border border-dashed border-slate-300">
                <svg class="w-12 h-12 text-slate-400 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                </svg>
                <p class="mt-4 text-sm font-semibold text-slate-700">Tidak ada dokumen ditemukan</p>
                <p class="text-xs text-slate-500 mt-1">Coba gunakan filter lain atau pilih kategori yang berbeda.</p>
            </div>
        @endforelse
    </div>

    <!-- Pagination -->
    <div class="mt-10">
        {{ $documents->links() }}
    </div>
</div>
@endsection