"use client";

import React, { useState, useEffect, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Download,
  Trash2,
  Edit3,
  Globe,
  Lock,
  Clock,
  CheckCircle,
  XCircle,
  FileText,
  Calendar,
  User,
  Building,
  HardDrive,
  ShieldCheck,
  AlertCircle,
  Eye,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";

interface DocumentDetail {
  id: number;
  title: string;
  description?: string;
  original_name: string;
  extension: string;
  mime_type: string;
  size_kb: number;
  category: { id: number; name: string };
  prodi?: { id: number; name: string; code: string };
  uploader: { id: number; name: string; role: string; nim?: string; nip?: string };
  owner: { id: number; name: string; role: string; nim?: string; nip?: string };
  visibility: string;
  verification_status?: string;
  archived_at?: string;
  created_at: string;
  links: { self: string; download: string };
}

interface Category {
  id: number;
  name: string;
}

export default function DocumentDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { user } = useAuth();
  const docId = params.id as string;

  const [doc, setDoc] = useState<DocumentDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Edit Metadata State
  const [editOpen, setEditOpen] = useState(false);
  const [categories, setCategories] = useState<Category[]>([]);
  const [editTitle, setEditTitle] = useState("");
  const [editDesc, setEditDesc] = useState("");
  const [editCategoryId, setEditCategoryId] = useState<number | "">("");
  const [updating, setUpdating] = useState(false);

  // Visibility Change State
  const [visChanging, setVisChanging] = useState(false);

  // Delete State
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [forceDelete, setForceDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const fetchDocument = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<DocumentDetail>(`/api/v1/documents/${docId}`);
      setDoc(res.data);
      setEditTitle(res.data.title);
      setEditDesc(res.data.description || "");
      setEditCategoryId(res.data.category.id);
    } catch (err: any) {
      console.error("Gagal memuat dokumen:", err);
      setError(err.message || "Berkas tidak ditemukan atau Anda tidak memiliki hak akses.");
    } finally {
      setLoading(false);
    }
  }, [docId]);

  useEffect(() => {
    fetchDocument();
    api.get<Category[]>("/api/v1/documents/categories")
      .then((res) => setCategories(res.data || []))
      .catch(() => {});
  }, [fetchDocument]);

  const handleDownload = async () => {
    if (!doc) return;
    try {
      const blob = await api.downloadBlob(doc.links.download);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = doc.original_name || `${doc.title}.${doc.extension}`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err: any) {
      alert("Gagal mengunduh berkas: " + (err.message || "Kesalahan jaringan."));
    }
  };

  const handleToggleVisibility = async () => {
    if (!doc) return;
    const nextVis = doc.visibility === "public" ? "private" : "public";
    if (nextVis === "public") {
      const confirm = window.confirm(
        "Mengubah visibilitas ke Publik memerlukan verifikasi ulang oleh BAAK/Super Admin sebelum muncul di katalog publik. Lanjutkan?"
      );
      if (!confirm) return;
    }

    setVisChanging(true);
    try {
      await api.patch(`/api/v1/documents/${doc.id}/visibility`, { visibility: nextVis });
      fetchDocument();
    } catch (err: any) {
      alert("Gagal mengubah visibilitas: " + (err.message || "Akses ditolak."));
    } finally {
      setVisChanging(false);
    }
  };

  const handleUpdateMetadata = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!doc || !editTitle.trim() || !editCategoryId) return;

    setUpdating(true);
    try {
      await api.patch(`/api/v1/documents/${doc.id}`, {
        title: editTitle.trim(),
        description: editDesc.trim(),
        category_id: Number(editCategoryId),
      });
      setEditOpen(false);
      fetchDocument();
    } catch (err: any) {
      alert("Gagal memperbarui metadata: " + (err.message || "Terjadi kesalahan."));
    } finally {
      setUpdating(false);
    }
  };

  const handleDelete = async () => {
    if (!doc) return;
    setDeleting(true);
    try {
      const url = forceDelete
        ? `/api/v1/documents/${doc.id}?force=true`
        : `/api/v1/documents/${doc.id}`;
      await api.delete(url);
      router.push("/berkas");
    } catch (err: any) {
      alert("Gagal menghapus berkas: " + (err.message || "Akses ditolak."));
      setDeleting(false);
    }
  };

  const formatSize = (kb: number) => {
    if (!kb) return "0 KB";
    if (kb < 1024) return `${kb} KB`;
    return `${(kb / 1024).toFixed(2)} MB`;
  };

  const getStatusBadge = (status?: string) => {
    if (!status) return null;
    switch (status) {
      case "approved":
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-emerald-100 text-emerald-800">
            <CheckCircle className="w-3.5 h-3.5" /> Disetujui BAAK
          </span>
        );
      case "pending":
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-amber-100 text-amber-800">
            <Clock className="w-3.5 h-3.5" /> Menunggu Verifikasi BAAK
          </span>
        );
      case "rejected":
        return (
          <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-semibold bg-red-100 text-red-800">
            <XCircle className="w-3.5 h-3.5" /> Ditolak BAAK
          </span>
        );
      default:
        return null;
    }
  };

  if (loading) {
    return (
      <div className="p-16 text-center">
        <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
        <p className="text-sm font-medium text-slate-500">Memuat rincian berkas...</p>
      </div>
    );
  }

  if (error || !doc) {
    return (
      <div className="max-w-xl mx-auto p-8 text-center bg-white rounded-2xl border border-slate-200 mt-10">
        <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-3" />
        <h3 className="text-lg font-bold text-slate-800">Berkas Tidak Ditemukan</h3>
        <p className="text-xs text-slate-500 mt-1">{error}</p>
        <button
          onClick={() => router.push("/berkas")}
          className="mt-6 inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-indigo-600 text-white text-xs font-semibold"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Kembali ke Berkas</span>
        </button>
      </div>
    );
  }

  const canEdit =
    user?.id === doc.uploader.id ||
    user?.role === "super_admin" ||
    user?.role === "baak";

  const canDelete =
    user?.id === doc.uploader.id ||
    user?.role === "super_admin";

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      {/* Back button */}
      <div>
        <button
          onClick={() => router.push("/berkas")}
          className="inline-flex items-center gap-2 text-xs font-semibold text-slate-500 hover:text-slate-800 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Kembali ke Daftar Berkas</span>
        </button>
      </div>

      {/* Main Document Header Card */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-6 shadow-sm">
        <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <span className="px-2.5 py-0.5 rounded-md text-xs font-bold uppercase tracking-wider bg-slate-100 text-slate-700">
                {doc.extension}
              </span>
              <span className="px-2.5 py-0.5 rounded-md text-xs font-medium bg-indigo-50 text-indigo-700">
                {doc.category.name}
              </span>
              {doc.visibility === "public" ? (
                <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-100 text-blue-800">
                  <Globe className="w-3 h-3" /> Publik
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-700">
                  <Lock className="w-3 h-3" /> Internal
                </span>
              )}
              {getStatusBadge(doc.verification_status)}
            </div>

            <h1 className="text-2xl font-extrabold text-slate-900 tracking-tight">{doc.title}</h1>
            <p className="text-xs text-slate-400 font-mono">{doc.original_name}</p>
          </div>

          {/* Action buttons */}
          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={handleDownload}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-xs shadow-sm shadow-indigo-200 transition-colors"
            >
              <Download className="w-4 h-4" />
              <span>Unduh Berkas</span>
            </button>

            {canEdit && (
              <>
                <button
                  onClick={() => setEditOpen(true)}
                  className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl border border-slate-200 hover:bg-slate-50 text-slate-700 font-medium text-xs transition-colors"
                >
                  <Edit3 className="w-3.5 h-3.5" />
                  <span>Ubah Data</span>
                </button>
                <button
                  onClick={handleToggleVisibility}
                  disabled={visChanging}
                  className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl border border-slate-200 hover:bg-slate-50 text-slate-700 font-medium text-xs transition-colors"
                >
                  <Eye className="w-3.5 h-3.5" />
                  <span>{doc.visibility === "public" ? "Jadikan Internal" : "Jadikan Publik"}</span>
                </button>
              </>
            )}

            {canDelete && (
              <button
                onClick={() => setDeleteOpen(true)}
                className="p-2 rounded-xl border border-red-200 text-red-600 hover:bg-red-50 transition-colors"
                title="Hapus Dokumen"
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        {/* Description section */}
        {doc.description && (
          <div className="mt-5 p-4 rounded-xl bg-slate-50 border border-slate-200/60 text-xs text-slate-600 leading-relaxed">
            <span className="font-semibold text-slate-800 block mb-1">Deskripsi:</span>
            {doc.description}
          </div>
        )}
      </div>

      {/* Verification Notice */}
      {doc.visibility === "public" && doc.verification_status === "pending" && (
        <div className="p-4 rounded-2xl bg-amber-50 border border-amber-200 flex items-start gap-3 text-amber-900 text-xs">
          <Clock className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
          <div>
            <h4 className="font-bold text-amber-800">Menunggu Verifikasi BAAK</h4>
            <p className="mt-0.5 text-amber-700">
              Dokumen ini diset ke <strong>Publik</strong> dan sedang menunggu verifikasi dari staf BAAK atau Super Admin.
              Sebelum disetujui, berkas hanya terlihat oleh pihak internal yang berwenang.
            </p>
          </div>
        </div>
      )}

      {/* Metadata Detail Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
        {/* Informasi Berkas */}
        <div className="bg-white rounded-2xl border border-slate-200/80 p-5 shadow-sm space-y-4">
          <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 flex items-center gap-2">
            <FileText className="w-4 h-4 text-indigo-600" /> Informasi Teknis Berkas
          </h3>

          <div className="space-y-3 text-xs divide-y divide-slate-100">
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">ID Dokumen</span>
              <span className="font-mono font-semibold text-slate-800">#{doc.id}</span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Ukuran File</span>
              <span className="font-semibold text-slate-800">{formatSize(doc.size_kb)}</span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Tipe Konten (MIME)</span>
              <span className="font-mono text-slate-800">{doc.mime_type}</span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Format Ekstensi</span>
              <span className="font-mono uppercase font-bold text-indigo-600">.{doc.extension}</span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Kategori Berkas</span>
              <span className="font-semibold text-slate-800">{doc.category.name}</span>
            </div>
          </div>
        </div>

        {/* Informasi Pemilik & Kepemilikan */}
        <div className="bg-white rounded-2xl border border-slate-200/80 p-5 shadow-sm space-y-4">
          <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 flex items-center gap-2">
            <User className="w-4 h-4 text-indigo-600" /> Kepemilikan & Program Studi
          </h3>

          <div className="space-y-3 text-xs divide-y divide-slate-100">
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Pengunggah (Uploader)</span>
              <div className="text-right">
                <p className="font-semibold text-slate-800">{doc.uploader.name}</p>
                <p className="text-[11px] text-slate-400 uppercase">{doc.uploader.role}</p>
              </div>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Subjek Berkas (Owner)</span>
              <div className="text-right">
                <p className="font-semibold text-slate-800">{doc.owner.name}</p>
                <p className="text-[11px] text-slate-400 uppercase">{doc.owner.role}</p>
              </div>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Program Studi Terkait</span>
              <span className="font-semibold text-slate-800">
                {doc.prodi ? `${doc.prodi.name} (${doc.prodi.code})` : "Fakultas / Umum"}
              </span>
            </div>
            <div className="flex justify-between py-1.5">
              <span className="text-slate-500">Waktu Unggah</span>
              <span className="text-slate-800 font-medium">
                {new Date(doc.created_at).toLocaleString("id-ID", {
                  dateStyle: "medium",
                  timeStyle: "short",
                })}
              </span>
            </div>
            {doc.archived_at && (
              <div className="flex justify-between py-1.5">
                <span className="text-slate-500">Tanggal Arsip Terdaftar</span>
                <span className="text-slate-800 font-medium">{doc.archived_at}</span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Retensi 8 Tahun Banner */}
      <div className="p-5 rounded-2xl bg-indigo-50/60 border border-indigo-100 flex items-start gap-3.5 text-xs text-indigo-900">
        <ShieldCheck className="w-5 h-5 text-indigo-600 flex-shrink-0 mt-0.5" />
        <div className="space-y-1">
          <h4 className="font-bold text-indigo-950">Kebijakan Retensi Berkas Digital FITE</h4>
          <p className="text-indigo-800/90 leading-relaxed">
            Sesuai peraturan fakultas dan SRS FITE, dokumen ini dilindungi kebijakan retensi otomatis selama 8 tahun kalender.
            Setelah melewati masa 8 tahun, berkas akan ditandai kadaluarsa sebelum dihapus permanen dari Cloud Storage
            setelah 90 hari masa tenggang pemulihan.
          </p>
        </div>
      </div>

      {/* Edit Metadata Modal */}
      {editOpen && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg p-6 animate-in fade-in zoom-in-95 duration-150">
            <h3 className="text-lg font-bold text-slate-900">Ubah Data Berkas</h3>
            <p className="text-xs text-slate-500 mt-1">Perbarui judul, kategori, dan deskripsi berkas.</p>

            <form onSubmit={handleUpdateMetadata} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Judul Berkas <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-200"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Kategori <span className="text-red-500">*</span>
                </label>
                <select
                  value={editCategoryId}
                  onChange={(e) => setEditCategoryId(Number(e.target.value))}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
                  required
                >
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Deskripsi
                </label>
                <textarea
                  rows={3}
                  value={editDesc}
                  onChange={(e) => setEditDesc(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-200"
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setEditOpen(false)}
                  className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={updating}
                  className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold"
                >
                  {updating ? "Menyimpan..." : "Simpan Perubahan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteOpen && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center mb-4">
              <AlertCircle className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Hapus Berkas?</h3>
            <p className="text-xs text-slate-500 mt-1">
              Anda akan menghapus: <strong className="text-slate-800">{doc.title}</strong>.
            </p>

            {user?.role === "super_admin" && (
              <div className="mt-4 p-3 rounded-xl bg-red-50/50 border border-red-200">
                <label className="flex items-center gap-2 cursor-pointer text-xs font-semibold text-red-900">
                  <input
                    type="checkbox"
                    checked={forceDelete}
                    onChange={(e) => setForceDelete(e.target.checked)}
                    className="rounded text-red-600 focus:ring-red-500"
                  />
                  <span>Hapus Permanen (Hard Delete GCS & Database)</span>
                </label>
              </div>
            )}

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                onClick={() => setDeleteOpen(false)}
                disabled={deleting}
                className="px-4 py-2 rounded-lg border border-slate-300 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-semibold flex items-center gap-2"
              >
                {deleting ? "Menghapus..." : "Hapus Berkas"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
