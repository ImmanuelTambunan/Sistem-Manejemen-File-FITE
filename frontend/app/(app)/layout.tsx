"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { MENU_ITEMS, hasAccess, Role } from "@/lib/permissions";
import { apiFetch } from "@/lib/api";
import {
  LayoutDashboard,
  FolderArchive,
  Globe,
  CheckSquare,
  Bell,
  Users,
  Tag,
  GraduationCap,
  HardDrive,
  User,
  LogOut,
  Menu,
  X,
  ShieldCheck,
  ShieldAlert,
  Activity,
  Loader2,
  ChevronDown,
} from "lucide-react";

const ICON_MAP: Record<string, React.ReactNode> = {
  LayoutDashboard: <LayoutDashboard className="w-5 h-5" />,
  FolderArchive: <FolderArchive className="w-5 h-5" />,
  Globe: <Globe className="w-5 h-5" />,
  CheckSquare: <CheckSquare className="w-5 h-5" />,
  Bell: <Bell className="w-5 h-5" />,
  Users: <Users className="w-5 h-5" />,
  Tag: <Tag className="w-5 h-5" />,
  GraduationCap: <GraduationCap className="w-5 h-5" />,
  HardDrive: <HardDrive className="w-5 h-5" />,
  User: <User className="w-5 h-5" />,
};

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { user, loading, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [unreadCount, setUnreadCount] = useState<number>(0);
  const [notifDropdownOpen, setNotifDropdownOpen] = useState(false);
  const [recentNotifs, setRecentNotifs] = useState<any[]>([]);

  // Guard routing
  useEffect(() => {
    if (!loading) {
      if (!user) {
        router.push("/login");
      } else if (user.must_change_password) {
        router.push("/ganti-sandi");
      }
    }
  }, [user, loading, router]);

  // Fetch unread notifications
  useEffect(() => {
    if (user && !user.must_change_password) {
      fetchNotifications();
      const interval = setInterval(fetchNotifications, 30000); // Polling every 30s
      return () => clearInterval(interval);
    }
  }, [user]);

  const fetchNotifications = async () => {
    try {
      const resCount = await apiFetch<{ unread: number }>("/notifications/unread-count");
      setUnreadCount(resCount.data?.unread || 0);

      const resList = await apiFetch<any[]>("/notifications?per_page=5");
      setRecentNotifs(resList.data || []);
    } catch {
      // ignore
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin text-blue-600 mx-auto mb-2" />
          <p className="text-sm text-slate-500">Memuat sesi pengguna...</p>
        </div>
      </div>
    );
  }

  if (!user) {
    return null;
  }

  // Check RBAC permission for current route
  const isAuthorized = hasAccess(user.role as Role, pathname);

  return (
    <div className="min-h-screen bg-slate-50 flex">
      {/* Mobile Sidebar Backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/50 backdrop-blur-sm lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed lg:sticky top-0 left-0 z-50 h-screen w-64 bg-slate-900 text-slate-200 flex flex-col transition-transform duration-300 ${
          sidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
        }`}
      >
        <div className="p-5 flex items-center justify-between border-b border-slate-800">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 bg-blue-600 text-white rounded-xl flex items-center justify-center font-bold">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <div>
              <span className="font-bold text-base text-white tracking-tight">SIFITE Arsip</span>
              <span className="block text-[10px] text-slate-400 uppercase tracking-widest font-semibold">
                FITE DEL
              </span>
            </div>
          </div>
          <button
            onClick={() => setSidebarOpen(false)}
            className="lg:hidden text-slate-400 hover:text-white"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* User Role Card in Sidebar */}
        <div className="p-4 mx-3 my-3 bg-slate-800/80 rounded-xl border border-slate-700/60">
          <p className="text-xs font-semibold text-white truncate">{user.name}</p>
          <div className="flex items-center gap-1.5 mt-1">
            <span className="inline-block w-2 h-2 rounded-full bg-emerald-400"></span>
            <span className="text-[11px] text-slate-300 font-medium">{user.role_label}</span>
          </div>
          {user.prodi && (
            <p className="text-[10px] text-slate-400 mt-0.5 truncate">{user.prodi.name}</p>
          )}
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-2 space-y-1 overflow-y-auto">
          {MENU_ITEMS.filter((item) => item.roles.includes(user.role as Role)).map((item) => {
            const active = pathname === item.href || (item.href !== "/dashboard" && pathname.startsWith(item.href));
            return (
              <Link
                key={item.href}
                href={item.href}
                onClick={() => setSidebarOpen(false)}
                className={`flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition-colors ${
                  active
                    ? "bg-blue-600 text-white shadow-sm"
                    : "text-slate-400 hover:text-white hover:bg-slate-800/60"
                }`}
              >
                {ICON_MAP[item.iconName] || <FolderArchive className="w-5 h-5" />}
                <span className="truncate">{item.title}</span>
                {item.href === "/notifikasi" && unreadCount > 0 && (
                  <span className="ml-auto px-2 py-0.5 text-xs font-bold rounded-full bg-red-500 text-white">
                    {unreadCount}
                  </span>
                )}
              </Link>
            );
          })}
        </nav>

        {/* Status & Logout Bottom */}
        <div className="p-3 border-t border-slate-800 space-y-1">
          <Link
            href="/status"
            className="w-full flex items-center gap-3 px-3.5 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white hover:bg-slate-800/60 transition-colors"
          >
            <Activity className="w-4 h-4 text-emerald-400" />
            <span>Status Sistem</span>
          </Link>
          <button
            onClick={() => logout()}
            className="w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium text-red-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
          >
            <LogOut className="w-5 h-5" />
            <span>Keluar Akun</span>
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Navbar */}
        <header className="sticky top-0 z-30 h-16 bg-white/90 backdrop-blur-md border-b border-slate-200/80 px-4 sm:px-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 rounded-lg text-slate-600 hover:bg-slate-100"
            >
              <Menu className="w-5 h-5" />
            </button>
            <h2 className="text-base font-semibold text-slate-800 hidden sm:block">
              Sistem Informasi Arsip Berkas Digital
            </h2>
          </div>

          <div className="flex items-center gap-4">
            {/* Notification Bell Dropdown */}
            <div className="relative">
              <button
                onClick={() => setNotifDropdownOpen(!notifDropdownOpen)}
                className="relative p-2 rounded-xl text-slate-600 hover:bg-slate-100 transition-colors"
              >
                <Bell className="w-5 h-5" />
                {unreadCount > 0 && (
                  <span className="absolute top-1.5 right-1.5 w-2.5 h-2.5 bg-red-500 rounded-full ring-2 ring-white"></span>
                )}
              </button>

              {notifDropdownOpen && (
                <div className="absolute right-0 mt-2 w-80 bg-white rounded-2xl shadow-xl border border-slate-200 py-2 z-50 animate-in fade-in zoom-in-95 duration-100">
                  <div className="px-4 py-2 border-b border-slate-100 flex items-center justify-between">
                    <span className="font-semibold text-xs text-slate-800">Notifikasi Terbaru</span>
                    <Link
                      href="/notifikasi"
                      onClick={() => setNotifDropdownOpen(false)}
                      className="text-xs text-blue-600 hover:underline"
                    >
                      Lihat Semua
                    </Link>
                  </div>

                  <div className="divide-y divide-slate-100 max-h-72 overflow-y-auto">
                    {recentNotifs.length === 0 ? (
                      <div className="p-4 text-center text-xs text-slate-400">
                        Tidak ada notifikasi baru
                      </div>
                    ) : (
                      recentNotifs.map((n) => (
                        <div
                          key={n.id}
                          className={`p-3 text-xs hover:bg-slate-50 transition-colors ${
                            !n.is_read ? "bg-blue-50/50 font-medium" : "text-slate-600"
                          }`}
                        >
                          <p className="font-semibold text-slate-900">{n.title}</p>
                          <p className="text-slate-600 mt-0.5 line-clamp-2">{n.message}</p>
                          <span className="text-[10px] text-slate-400 mt-1 block">
                            {new Date(n.created_at).toLocaleString("id-ID")}
                          </span>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* Profile Dropdown Shortcut */}
            <Link
              href="/profil"
              className="flex items-center gap-2 p-1.5 pr-3 rounded-xl hover:bg-slate-100 transition-colors"
            >
              <div className="w-8 h-8 rounded-lg bg-blue-100 text-blue-700 font-bold flex items-center justify-center text-xs">
                {user.name.charAt(0)}
              </div>
              <div className="hidden md:block text-left text-xs">
                <p className="font-semibold text-slate-800 leading-tight">{user.name}</p>
                <p className="text-slate-400 text-[10px]">{user.role_label}</p>
              </div>
            </Link>
          </div>
        </header>

        {/* Content Body */}
        <main className="flex-1 p-4 sm:p-6 md:p-8 max-w-7xl w-full mx-auto">
          {!isAuthorized ? (
            <div className="p-8 bg-white rounded-3xl border border-red-200 text-center shadow-sm">
              <div className="w-16 h-16 bg-red-50 text-red-600 rounded-full flex items-center justify-center mx-auto mb-4">
                <ShieldAlert className="w-8 h-8" />
              </div>
              <h3 className="text-xl font-bold text-slate-900 mb-2">Akses Ditolak (403 Forbidden)</h3>
              <p className="text-slate-600 text-sm max-w-md mx-auto mb-6">
                Peran Anda ({user.role_label}) tidak memiliki wewenang untuk membuka modul halaman ini.
              </p>
              <Link
                href="/dashboard"
                className="px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-medium text-sm transition-colors shadow-sm"
              >
                Kembali ke Dashboard
              </Link>
            </div>
          ) : (
            children
          )}
        </main>
      </div>
    </div>
  );
}
