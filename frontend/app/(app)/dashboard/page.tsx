"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import {
  FileText,
  Clock,
  CheckCircle,
  XCircle,
  HardDrive,
  Users,
  TrendingUp,
  Upload,
  ArrowRight,
  ShieldAlert,
  BarChart3,
  Calendar,
} from "lucide-react";
import { useAuth } from "@/lib/auth";
import { api } from "@/lib/api";
import UploadModal from "@/components/UploadModal";

interface SummaryData {
  role: string;
  cards: Record<string, number>;
  documents_by_prodi?: { prodi: string; total: number }[];
}

interface RecentDocument {
  id: number;
  title: string;
  file_name?: string;
  original_name?: string;
  file_size_bytes?: number;
  size_kb?: number;
  mime_type: string;
  visibility: string;
  verification_status: string;
  created_at: string;
  category?: { name: string; code: string };
  prodi?: { name: string; code: string };
}

export default function DashboardPage() {
  const { user } = useAuth();
  const [summary, setSummary] = useState<SummaryData | null>(null);
  const [recentDocs, setRecentDocs] = useState<RecentDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploadOpen, setUploadOpen] = useState(false);

  const fetchDashboardData = async () => {
    try {
      setLoading(true);
      const [sumRes, docsRes] = await Promise.all([
        api.get<SummaryData>("/api/v1/dashboard/summary"),
        api.get<RecentDocument[]>("/api/v1/documents?per_page=6&page=1"),
      ]);

      setSummary(sumRes.data);
      setRecentDocs(docsRes.data || []);
    } catch (err) {
      console.error("Gagal memuat data dasbor:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const formatSize = (bytes: number) => {
    if (!bytes) return "0 B";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "approved":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-100 text-emerald-800">
            <CheckCircle className="w-3 h-3" /> Disetujui
          </span>
        );
      case "pending":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800">
            <Clock className="w-3 h-3" /> Menunggu
          </span>
        );
      case "rejected":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-100 text-red-800">
            <XCircle className="w-3 h-3" /> Ditolak
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-800">
            Draft
          </span>
        );
    }
  };

  const renderRoleCards = () => {
    if (!summary || !summary.cards) return null;
    const cards = summary.cards;

    if (user?.role === "super_admin") {
      return (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Total Berkas</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.total_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Seluruh berkas terdaftar</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <FileText className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Antrean Verifikasi</p>
              <h4 className="text-2xl font-bold text-amber-600 mt-1">{cards.pending_verifications ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Perlu ditinjau BAAK</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center">
              <Clock className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Pengguna Aktif</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.total_users ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Akun dosen & mahasiswa</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <Users className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Kapasitas Storage</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">
                {cards.storage_used_mb ?? 0} <span className="text-sm font-medium text-slate-500">MB</span>
              </h4>
              <p className="text-xs text-slate-400 mt-1">Cloud Storage GCS</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center">
              <HardDrive className="w-6 h-6" />
            </div>
          </div>
        </div>
      );
    }

    if (user?.role === "baak") {
      return (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          <div className="p-5 rounded-2xl bg-white border border-amber-200 bg-gradient-to-br from-amber-50/50 to-white shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-amber-700">Perlu Verifikasi</p>
              <h4 className="text-3xl font-extrabold text-amber-800 mt-1">{cards.pending_verifications ?? 0}</h4>
              <p className="text-xs text-amber-600 mt-1">Menunggu persetujuan</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-amber-100 text-amber-700 flex items-center justify-center">
              <Clock className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Total Berkas Fakultas</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.total_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Semua berkas operasional</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <FileText className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Unggahan Hari Ini</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.uploads_today ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Aktivitas 24 jam terakhir</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center">
              <TrendingUp className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Unggahan Saya</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.my_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Berkas terunggah via BAAK</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <CheckCircle className="w-6 h-6" />
            </div>
          </div>
        </div>
      );
    }

    if (user?.role === "dekan") {
      return (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-5">
          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Total Berkas Fakultas</p>
              <h4 className="text-3xl font-bold text-slate-900 mt-1">{cards.total_faculty_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Semua prodi FITE</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <FileText className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Dokumen Akreditasi</p>
              <h4 className="text-3xl font-bold text-indigo-600 mt-1">{cards.accreditation_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Kategori Akreditasi FITE</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <BarChart3 className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Dokumen Saya</p>
              <h4 className="text-3xl font-bold text-slate-900 mt-1">{cards.my_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">SK / Arsip Dekanat</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center">
              <Users className="w-6 h-6" />
            </div>
          </div>
        </div>
      );
    }

    if (user?.role === "kaprodi") {
      return (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Prodi</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.prodi_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">{user.prodi?.name || "Program Studi"}</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <FileText className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Mahasiswa</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.prodi_student_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Skripsi, KP, KRS prodi</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center">
              <Users className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Dosen FITE</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.faculty_lecturer_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">RPS, Silabus, Penelitian</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <BarChart3 className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Saya</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.my_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Dokumen pribadi kaprodi</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center">
              <CheckCircle className="w-6 h-6" />
            </div>
          </div>
        </div>
      );
    }

    if (user?.role === "dosen") {
      return (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-5">
          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Saya</p>
              <h4 className="text-3xl font-bold text-slate-900 mt-1">{cards.my_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">RPS, Modul, Berkas Dosen</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <FileText className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Berkas Mahasiswa Prodi</p>
              <h4 className="text-3xl font-bold text-blue-600 mt-1">{cards.student_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">{user.prodi?.name || "Program Studi"}</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center">
              <Users className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Status Verifikasi</p>
              <h4 className="text-3xl font-bold text-amber-600 mt-1">{cards.pending_or_rejected_documents ?? 0}</h4>
              <p className="text-xs text-slate-400 mt-1">Menunggu / Perlu revisi</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center">
              <Clock className="w-6 h-6" />
            </div>
          </div>
        </div>
      );
    }

    // Default: Mahasiswa
    return (
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5">
        <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Total Berkas Saya</p>
            <h4 className="text-2xl font-bold text-slate-900 mt-1">{cards.my_documents ?? 0}</h4>
            <p className="text-xs text-slate-400 mt-1">Dokumen tersimpan</p>
          </div>
          <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
            <FileText className="w-6 h-6" />
          </div>
        </div>

        <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Menunggu Verifikasi</p>
            <h4 className="text-2xl font-bold text-amber-600 mt-1">{cards.pending_documents ?? 0}</h4>
            <p className="text-xs text-slate-400 mt-1">Sedang ditinjau BAAK</p>
          </div>
          <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center">
            <Clock className="w-6 h-6" />
          </div>
        </div>

        <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Disetujui</p>
            <h4 className="text-2xl font-bold text-emerald-600 mt-1">{cards.approved_documents ?? 0}</h4>
            <p className="text-xs text-slate-400 mt-1">Terverifikasi resmi</p>
          </div>
          <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
            <CheckCircle className="w-6 h-6" />
          </div>
        </div>

        <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wider text-slate-500">Ditolak</p>
            <h4 className="text-2xl font-bold text-red-600 mt-1">{cards.rejected_documents ?? 0}</h4>
            <p className="text-xs text-slate-400 mt-1">Lihat alasan penolakan</p>
          </div>
          <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center">
            <XCircle className="w-6 h-6" />
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-7">
      {/* Welcome Banner */}
      <div className="relative overflow-hidden rounded-3xl bg-gradient-to-r from-indigo-900 via-indigo-800 to-slate-900 p-8 text-white shadow-xl">
        <div className="relative z-10 max-w-2xl">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/10 backdrop-blur-md text-xs font-medium text-indigo-200 mb-3 border border-white/10">
            <Calendar className="w-3.5 h-3.5" />
            <span>Sistem Informasi Arsip Digital FITE</span>
          </div>
          <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
            Selamat Datang, {user?.name || "Pengguna"}!
          </h2>
          <p className="text-sm text-indigo-100 mt-2 leading-relaxed">
            Anda masuk sebagai <strong className="text-white underline">{user?.role_label || user?.role}</strong>
            {user?.prodi ? ` di Program Studi ${user.prodi.name}` : " Fakultas Informatika & Teknik Elektro"}.
            Kelola dan arsipkan dokumen akademik dengan aman sesuai siklus retensi 8 tahun.
          </p>

          <div className="flex flex-wrap items-center gap-3 mt-6">
            <button
              onClick={() => setUploadOpen(true)}
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white text-indigo-900 font-semibold text-sm shadow-md hover:bg-indigo-50 transition-colors"
            >
              <Upload className="w-4 h-4 text-indigo-600" />
              <span>Unggah Berkas</span>
            </button>
            <Link
              href="/berkas"
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 text-white font-medium text-sm border border-white/20 backdrop-blur-md transition-colors"
            >
              <span>Jelajahi Berkas</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
            {(user?.role === "baak" || user?.role === "super_admin") && (
              <Link
                href="/verifikasi"
                className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-amber-500 hover:bg-amber-600 text-white font-semibold text-sm shadow-md transition-colors"
              >
                <Clock className="w-4 h-4" />
                <span>Panel Verifikasi</span>
              </Link>
            )}
          </div>
        </div>

        {/* Decorative background shapes */}
        <div className="absolute -right-10 -bottom-10 w-64 h-64 rounded-full bg-indigo-500/10 blur-3xl pointer-events-none" />
        <div className="absolute right-32 top-0 w-48 h-48 rounded-full bg-blue-500/10 blur-2xl pointer-events-none" />
      </div>

      {/* KPI Cards */}
      {renderRoleCards()}

      {/* Distribution by Prodi (if Super Admin, Dekan, BAAK) */}
      {summary?.documents_by_prodi && summary.documents_by_prodi.length > 0 && (
        <div className="bg-white rounded-2xl border border-slate-200/80 p-6 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="text-base font-bold text-slate-800">Distribusi Berkas per Program Studi</h3>
              <p className="text-xs text-slate-500">Sebaran total dokumen pada 3 program studi FITE</p>
            </div>
            <BarChart3 className="w-5 h-5 text-slate-400" />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {summary.documents_by_prodi.map((p) => (
              <div key={p.prodi} className="p-4 rounded-xl bg-slate-50 border border-slate-200/60">
                <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">{p.prodi}</p>
                <div className="flex items-baseline justify-between mt-2">
                  <span className="text-2xl font-bold text-indigo-700">{p.total}</span>
                  <span className="text-xs text-slate-400">dokumen</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent Uploads Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100">
          <div>
            <h3 className="text-base font-bold text-slate-800">Unggahan Terakhir</h3>
            <p className="text-xs text-slate-500">Berkas terbaru yang dapat Anda akses</p>
          </div>
          <Link
            href="/berkas"
            className="text-xs font-semibold text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
          >
            <span>Semua Berkas</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Link>
        </div>

        {recentDocs.length === 0 ? (
          <div className="p-12 text-center">
            <FileText className="w-12 h-12 text-slate-300 mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-600">Belum ada berkas terunggah.</p>
            <p className="text-xs text-slate-400 mt-1">Unggah dokumen pertama Anda melalui tombol di atas.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Judul Berkas</th>
                  <th className="px-6 py-3.5">Kategori</th>
                  <th className="px-6 py-3.5">Ukuran</th>
                  <th className="px-6 py-3.5">Visibilitas</th>
                  <th className="px-6 py-3.5">Status</th>
                  <th className="px-6 py-3.5 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {recentDocs.map((doc) => (
                  <tr key={doc.id} className="hover:bg-slate-50/50 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className="w-9 h-9 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center flex-shrink-0">
                          <FileText className="w-4 h-4" />
                        </div>
                        <div>
                          <Link
                            href={`/berkas/${doc.id}`}
                            className="font-semibold text-slate-900 hover:text-indigo-600 line-clamp-1 transition-colors"
                          >
                            {doc.title}
                          </Link>
                          <span className="text-xs text-slate-400 font-mono">{doc.file_name || doc.original_name}</span>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-block px-2.5 py-1 rounded-md text-xs font-medium bg-slate-100 text-slate-700">
                        {doc.category?.name || "Umum"}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-xs font-medium text-slate-500">
                      {formatSize(doc.file_size_bytes || (doc.size_kb ? doc.size_kb * 1024 : 0))}
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={`inline-block px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          doc.visibility === "public"
                            ? "bg-blue-100 text-blue-800"
                            : "bg-slate-100 text-slate-700"
                        }`}
                      >
                        {doc.visibility === "public" ? "Publik" : "Internal"}
                      </span>
                    </td>
                    <td className="px-6 py-4">{getStatusBadge(doc.verification_status)}</td>
                    <td className="px-6 py-4 text-right">
                      <Link
                        href={`/berkas/${doc.id}`}
                        className="text-xs font-semibold text-indigo-600 hover:text-indigo-800 transition-colors"
                      >
                        Lihat Detail
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Upload Modal */}
      <UploadModal
        isOpen={uploadOpen}
        onClose={() => setUploadOpen(false)}
        onSuccess={() => {
          fetchDashboardData();
        }}
      />
    </div>
  );
}
