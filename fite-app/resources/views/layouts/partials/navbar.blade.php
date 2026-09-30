<header class="sticky top-0 z-50 bg-white/95 backdrop-blur-md border-b border-slate-200">
    <!-- Top Header -->
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between h-20">
            <!-- Brand Logo & Nama Fakultas -->
            <div class="flex items-center gap-3">
                <a href="{{ route('home') }}" class="flex items-center gap-3 group">
                    <div class="w-11 h-11 rounded-xl bg-gradient-to-tr from-blue-900 to-indigo-600 flex items-center justify-center text-white shadow-md shadow-blue-900/20 group-hover:scale-105 transition-transform">
                        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"></path>
                        </svg>
                    </div>
                    <div>
                        <span class="block text-xl font-black tracking-tight text-slate-900 leading-none">REPOSITORI <span class="text-blue-700">FITE</span></span>
                        <span class="text-xs text-slate-500 font-medium">Fakultas Informatika & Teknik Elektro</span>
                    </div>
                </a>
            </div>

            <!-- Auth Navigation & Action Buttons -->
            <div class="flex items-center gap-3">
                @auth
                    <!-- Profile Dropdown -->
                    <div class="flex items-center gap-3 pl-4 border-l border-slate-200">
                        <div class="text-right hidden sm:block">
                            <p class="text-sm font-semibold text-slate-800 leading-none">{{ Auth::user()->name }}</p>
                            <span class="inline-flex items-center px-2 py-0.5 mt-1 rounded text-[10px] font-bold tracking-wide uppercase bg-blue-100 text-blue-800">
                                {{ Auth::user()->role->name ?? 'User' }}
                            </span>
                        </div>
                        <a href="{{ route('dashboard') }}" class="inline-flex items-center justify-center h-10 px-4 text-sm font-semibold text-white bg-slate-900 hover:bg-slate-800 rounded-lg transition-colors shadow-sm">
                            Dashboard
                        </a>
                        <form method="POST" action="{{ route('logout') }}" class="inline">
                            @csrf
                            <button type="submit" class="p-2 text-slate-400 hover:text-rose-600 rounded-lg transition-colors" title="Keluar">
                                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"></path>
                                </svg>
                            </button>
                        </form>
                    </div>
                @else
                    <a href="{{ route('login') }}" class="inline-flex items-center justify-center h-10 px-4 text-sm font-semibold text-slate-700 hover:text-blue-700 transition-colors">
                        Masuk
                    </a>
                    <a href="{{ route('register') }}" class="inline-flex items-center justify-center h-10 px-5 text-sm font-semibold text-white bg-blue-700 hover:bg-blue-800 rounded-lg shadow-sm shadow-blue-700/20 transition-all hover:shadow-md">
                        Daftar Akun
                    </a>
                @endauth
            </div>
        </div>

        <!-- Category Pill Navigation (Traveloka Style Horizontal Tab) -->
        <nav class="flex items-center gap-2 py-2.5 overflow-x-auto scrollbar-none border-t border-slate-100">
            <a href="{{ route('home') }}" 
               class="px-4 py-2 rounded-full text-xs font-semibold whitespace-nowrap transition-all {{ !request('category') ? 'bg-blue-700 text-white shadow-sm shadow-blue-700/25' : 'text-slate-600 hover:bg-slate-100' }}">
                Semua Berkas
            </a>
            @foreach($categories as $category)
                <a href="{{ route('home', ['category' => $category->slug]) }}" 
                   class="px-4 py-2 rounded-full text-xs font-semibold whitespace-nowrap transition-all {{ request('category') == $category->slug ? 'bg-blue-700 text-white shadow-sm shadow-blue-700/25' : 'text-slate-600 hover:bg-slate-100' }}">
                    {{ $category->name }}
                </a>
            @endforeach
        </nav>
    </div>
</header>