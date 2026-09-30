<footer class="bg-slate-950 text-slate-400 border-t border-slate-900 mt-20">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div class="grid grid-cols-1 md:grid-cols-4 gap-8">
            <div class="md:col-span-2">
                <span class="text-lg font-bold tracking-tight text-white">REPOSITORI DIGITAL FITE</span>
                <p class="mt-3 text-sm text-slate-400 max-w-md leading-relaxed">
                    Pusat repositori terintegrasi Fakultas Informatika dan Teknik Elektro. Mendukung penyimpanan berkas ilmiah, materi pembelajaran, modul, serta source code proyek tugas akhir mahasiswa.
                </p>
            </div>
            <div>
                <h4 class="text-xs font-bold uppercase tracking-wider text-slate-300">Program Studi</h4>
                <ul class="mt-4 space-y-2 text-sm">
                    <li>S1 Informatika</li>
                    <li>S1 Sistem Informasi</li>
                    <li>D3/D4 Teknik Elektro</li>
                </ul>
            </div>
            <div>
                <h4 class="text-xs font-bold uppercase tracking-wider text-slate-300">Akses Cepat</h4>
                <ul class="mt-4 space-y-2 text-sm">
                    <li><a href="{{ route('login') }}" class="hover:text-white transition-colors">Portal Akademik</a></li>
                    <li><a href="{{ route('home', ['category' => 'source-code-ta']) }}" class="hover:text-white transition-colors">Source Code Tugas Akhir</a></li>
                    <li><a href="{{ route('home', ['category' => 'mata-kuliah']) }}" class="hover:text-white transition-colors">Bahan Ajar Perkuliahan</a></li>
                </ul>
            </div>
        </div>
        <div class="border-t border-slate-800/80 mt-10 pt-6 flex flex-col sm:flex-row items-center justify-between text-xs text-slate-500">
            <p>&copy; {{ date('Y') }} Fakultas Informatika dan Teknik Elektro. Seluruh Hak Cipta Dilindungi.</p>
            <p>Platform Repositori Dokumen FITE v1.0</p>
        </div>
    </div>
</footer>