"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  FileText,
  Search,
  Filter,
  Download,
  Trash2,
  ExternalLink,
  Upload,
  CheckCircle,
  Clock,
  XCircle,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  RefreshCw,
  FolderOpen,
} from "lucide-react";
import { api, getApiUrl } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import UploadModal from "@/components/UploadModal";

interface Category {
  id: number;
  name: string;
  code: string;
}

interface DocumentItem {
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
  created_at: string;
  links: { self: string; download: string };
}

interface Meta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export default function FileExplorerPage() {
  const { user } = useAuth();
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 20, total: 0, total_pages: 1 });
  const [loading, setLoading] = useState(true);

  // Filters
  const [q, setQ] = useState("");
  const [prodi, setProdi] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [extension, setExtension] = useState("");
  const [visibility, setVisibility] = useState("");
  const [sort, setSort] = useState("-created_at");
  const [page, setPage] = useState(1);

  const [categories, setCategories] = useState<Category[]>([]);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DocumentItem | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [forceDelete, setForceDelete] = useState(false);

  // Load categories
  useEffect(() => {
    api.get<Category[]>("/api/v1/documents/categories")
      .then((res) => setCategories(res.data || []))
      .catch((err) => console.error("Gagal memuat kategori:", err));
  }, []);

  const fetchDocuments = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (q.trim()) params.append("q", q.trim());
      if (prodi) params.append("prodi", prodi);
      if (categoryId) params.append("category_id", categoryId);
      if (extension) params.append("extension", extension);
      if (visibility) params.append("visibility", visibility);
      if (sort) params.append("sort", sort);
      params.append("page", String(page));
      params.append("per_page", "15");

      const res = await api.get<DocumentItem[]>(
        `/api/v1/documents?${params.toString()}`
      );
      setDocuments(res.data || []);
      if (res.meta) {
        setMeta(res.meta);
      }
    } catch (err) {
      console.error("Gagal memuat dokumen:", err);
    } finally {
      setLoading(false);
    }
  }, [q, prodi, categoryId, extension, visibility, sort, page]);

  useEffect(() => {
    fetchDocuments();
  }, [fetchDocuments]);

  const handleDownload = async (doc: DocumentItem) => {
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

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const url = forceDelete
        ? `/api/v1/documents/${deleteTarget.id}?force=true`
        : `/api/v1/documents/${deleteTarget.id}`;
      await api.delete(url);
      setDeleteTarget(null);
      setForceDelete(false);
      fetchDocuments();
    } catch (err: any) {
      alert("Gagal menghapus berkas: " + (err.message || "Akses ditolak."));
    } finally {
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
        return null;
    }
  };

  const getFileIcon = (ext: string) => {
    const e = ext.toLowerCase();
    let color = "text-indigo-600 bg-indigo-50";
    if (e === "pdf") color = "text-red-600 bg-red-50";
    else if (e === "docx" || e === "doc") color = "text-blue-600 bg-blue-50";
    else if (e === "xlsx" || e === "xls") color = "text-emerald-600 bg-emerald-50";
    else if (e === "pptx" || e === "ppt") color = "text-amber-600 bg-amber-50";
    else if (e === "zip") color = "text-purple-600 bg-purple-50";
    else if (["png", "jpg", "jpeg"].includes(e)) color = "text-teal-600 bg-teal-50";

    return (
      <div className={`w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0 font-bold uppercase text-xs ${color}`}>
        {e.slice(0, 3)}
      </div>
    );
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Manajemen Berkas</h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Jelajahi, cari, dan kelola arsip dokumen digital Fakultas FITE
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => fetchDocuments()}
            className="p-2.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-slate-600 transition-colors"
            title="Muat Ulang"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />
          </button>
          <button
            onClick={() => setUploadOpen(true)}
            className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-sm shadow-sm shadow-indigo-200 transition-colors"
          >
            <Upload className="w-4 h-4" />
            <span>Unggah Berkas</span>
          </button>
        </div>
      </div>

      {/* Search & Filter Toolbar */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-sm space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
          {/* Search Bar */}
          <div className="md:col-span-2 relative">
            <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={q}
              onChange={(e) => {
                setQ(e.target.value);
                setPage(1);
              }}
              placeholder="Cari judul berkas, nama file, pemilik..."
              className="w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
            />
          </div>

          {/* Prodi Filter */}
          <div>
            <select
              value={prodi}
              onChange={(e) => {
                setProdi(e.target.value);
                setPage(1);
              }}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
            >
              <option value="">Semua Program Studi</option>
              <option value="IF">Informatika (IF)</option>
              <option value="SI">Sistem Informasi (SI)</option>
              <option value="TE">Teknik Elektro (TE)</option>
            </select>
          </div>

          {/* Category Filter */}
          <div>
            <select
              value={categoryId}
              onChange={(e) => {
                setCategoryId(e.target.value);
                setPage(1);
              }}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
            >
              <option value="">Semua Kategori</option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Secondary Filter Row */}
        <div className="flex flex-wrap items-center gap-3 pt-2 border-t border-slate-100 text-xs">
          <span className="font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1">
            <Filter className="w-3.5 h-3.5" /> Filter:
          </span>

          <select
            value={visibility}
            onChange={(e) => {
              setVisibility(e.target.value);
              setPage(1);
            }}
            className="px-2.5 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700"
          >
            <option value="">Semua Visibilitas</option>
            <option value="private">Internal</option>
            <option value="public_approved">Publik (Disetujui)</option>
          </select>

          <select
            value={extension}
            onChange={(e) => {
              setExtension(e.target.value);
              setPage(1);
            }}
            className="px-2.5 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700"
          >
            <option value="">Semua Ekstensi</option>
            <option value="pdf">PDF</option>
            <option value="docx">DOCX (Word)</option>
            <option value="xlsx">XLSX (Excel)</option>
            <option value="pptx">PPTX (PowerPoint)</option>
            <option value="zip">ZIP (Arsip)</option>
            <option value="png">PNG (Gambar)</option>
            <option value="jpg">JPG (Gambar)</option>
          </select>

          <select
            value={sort}
            onChange={(e) => {
              setSort(e.target.value);
              setPage(1);
            }}
            className="px-2.5 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700 ml-auto"
          >
            <option value="-created_at">Terbaru</option>
            <option value="created_at">Terlama</option>
            <option value="title">Judul (A-Z)</option>
            <option value="-title">Judul (Z-A)</option>
            <option value="-size_kb">Ukuran Terbesar</option>
            <option value="size_kb">Ukuran Terkecil</option>
          </select>
        </div>
      </div>

      {/* Document Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat berkas...</p>
          </div>
        ) : documents.length === 0 ? (
          <div className="p-16 text-center">
            <FolderOpen className="w-14 h-14 text-slate-300 mx-auto mb-3" />
            <h4 className="text-base font-semibold text-slate-700">Tidak ada berkas yang cocok</h4>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              Coba sesuaikan kata kunci pencarian atau ubah filter untuk menemukan berkas yang diinginkan.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Judul & Nama Berkas</th>
                  <th className="px-6 py-3.5">Kategori</th>
                  <th className="px-6 py-3.5">Pengunggah / Pemilik</th>
                  <th className="px-6 py-3.5">Prodi</th>
                  <th className="px-6 py-3.5">Ukuran</th>
                  <th className="px-6 py-3.5">Visibilitas</th>
                  <th className="px-6 py-3.5">Status</th>
                  <th className="px-6 py-3.5 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {documents.map((doc) => (
                  <tr key={doc.id} className="hover:bg-slate-50/60 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        {getFileIcon(doc.extension)}
                        <div className="max-w-xs sm:max-w-sm">
                          <Link
                            href={`/berkas/${doc.id}`}
                            className="font-semibold text-slate-900 hover:text-indigo-600 line-clamp-1 transition-colors"
                          >
                            {doc.title}
                          </Link>
                          <span className="text-xs text-slate-400 font-mono line-clamp-1">
                            {doc.original_name}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="inline-block px-2.5 py-1 rounded-md text-xs font-medium bg-slate-100 text-slate-700">
                        {doc.category?.name || "Umum"}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <div>
                        <p className="text-xs font-semibold text-slate-800">{doc.uploader?.name}</p>
                        <p className="text-[11px] text-slate-400">
                          {doc.uploader?.role?.toUpperCase()}
                          {doc.uploader?.nim ? ` • ${doc.uploader.nim}` : ""}
                        </p>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs font-medium text-slate-600">
                        {doc.prodi?.code || "-"}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-xs font-medium text-slate-500">
                      {formatSize(doc.size_kb)}
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
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => handleDownload(doc)}
                          className="p-1.5 text-slate-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
                          title="Unduh Berkas"
                        >
                          <Download className="w-4 h-4" />
                        </button>
                        <Link
                          href={`/berkas/${doc.id}`}
                          className="p-1.5 text-slate-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
                          title="Detail Berkas"
                        >
                          <ExternalLink className="w-4 h-4" />
                        </Link>
                        {/* Hapus berkas jika uploader / admin */}
                        {(user?.id === doc.uploader.id || user?.role === "super_admin") && (
                          <button
                            onClick={() => {
                              setDeleteTarget(doc);
                              setForceDelete(false);
                            }}
                            className="p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                            title="Hapus Berkas"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination Footer */}
        {meta.total_pages > 1 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 bg-slate-50/40 text-xs">
            <span className="text-slate-500">
              Menampilkan {documents.length} dari {meta.total} berkas (Halaman {meta.page} dari {meta.total_pages})
            </span>
            <div className="flex items-center gap-1.5">
              <button
                disabled={meta.page <= 1}
                onClick={() => setPage((p) => Math.max(p - 1, 1))}
                className="p-1.5 rounded-lg border border-slate-200 text-slate-600 hover:bg-white disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              {Array.from({ length: meta.total_pages }, (_, i) => i + 1)
                .filter((p) => p === 1 || p === meta.total_pages || Math.abs(p - meta.page) <= 1)
                .map((p, idx, arr) => (
                  <React.Fragment key={p}>
                    {idx > 0 && arr[idx - 1] !== p - 1 && <span className="px-1 text-slate-400">...</span>}
                    <button
                      onClick={() => setPage(p)}
                      className={`w-7 h-7 rounded-lg font-medium transition-colors ${
                        p === meta.page
                          ? "bg-indigo-600 text-white"
                          : "text-slate-600 hover:bg-slate-200/60"
                      }`}
                    >
                      {p}
                    </button>
                  </React.Fragment>
                ))}
              <button
                disabled={meta.page >= meta.total_pages}
                onClick={() => setPage((p) => Math.min(p + 1, meta.total_pages))}
                className="p-1.5 rounded-lg border border-slate-200 text-slate-600 hover:bg-white disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center mb-4">
              <AlertCircle className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Hapus Berkas?</h3>
            <p className="text-xs text-slate-500 mt-1">
              Anda akan menghapus dokumen: <strong className="text-slate-800">{deleteTarget.title}</strong>.
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
                  <span>Hapus Permanen dari Cloud Storage (Hard Delete)</span>
                </label>
                <p className="text-[11px] text-red-600 mt-1 ml-6">
                  Perhatian: Tindakan ini akan langsung melenyapkan berkas dari GCS dan database tanpa masa tenggang 90 hari.
                </p>
              </div>
            )}

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                onClick={() => setDeleteTarget(null)}
                disabled={deleting}
                className="px-4 py-2 rounded-lg border border-slate-300 text-sm font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-semibold shadow-sm shadow-red-200 flex items-center gap-2"
              >
                {deleting ? (
                  <>
                    <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    <span>Menghapus...</span>
                  </>
                ) : (
                  <span>Hapus</span>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Upload Modal */}
      <UploadModal
        isOpen={uploadOpen}
        onClose={() => setUploadOpen(false)}
        onSuccess={() => fetchDocuments()}
      />
    </div>
  );
}
