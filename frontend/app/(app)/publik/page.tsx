"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  Globe,
  Search,
  Download,
  Calendar,
  Building,
  FileText,
  Filter,
  ExternalLink,
  FolderOpen,
} from "lucide-react";
import { api } from "@/lib/api";

interface Category {
  id: number;
  name: string;
}

interface PublicDocument {
  id: number;
  title: string;
  description?: string;
  original_name: string;
  extension: string;
  mime_type: string;
  size_kb: number;
  category: { id: number; name: string };
  prodi?: { id: number; name: string; code: string };
  uploader: { name: string; role: string };
  created_at: string;
  links: { self: string; download: string };
}

export default function PublicExplorerPage() {
  const [documents, setDocuments] = useState<PublicDocument[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);

  // Filters
  const [q, setQ] = useState("");
  const [prodi, setProdi] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [extension, setExtension] = useState("");

  const fetchPublicDocs = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (q.trim()) params.append("q", q.trim());
      if (prodi) params.append("prodi", prodi);
      if (categoryId) params.append("category_id", categoryId);
      if (extension) params.append("extension", extension);
      params.append("limit", "50");

      const res = await api.get<PublicDocument[]>(
        `/api/v1/documents/public?${params.toString()}`
      );
      setDocuments(res.data || []);
    } catch (err) {
      console.error("Gagal memuat berkas publik:", err);
    } finally {
      setLoading(false);
    }
  }, [q, prodi, categoryId, extension]);

  useEffect(() => {
    fetchPublicDocs();
    api.get<Category[]>("/api/v1/documents/categories")
      .then((res) => setCategories(res.data || []))
      .catch(() => {});
  }, [fetchPublicDocs]);

  const handleDownload = async (doc: PublicDocument) => {
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

  const formatSize = (kb: number) => {
    if (!kb) return "0 KB";
    if (kb < 1024) return `${kb} KB`;
    return `${(kb / 1024).toFixed(2)} MB`;
  };

  const getExtensionColor = (ext: string) => {
    const e = ext.toLowerCase();
    if (e === "pdf") return "bg-red-50 text-red-700 border-red-200";
    if (e === "docx" || e === "doc") return "bg-blue-50 text-blue-700 border-blue-200";
    if (e === "xlsx" || e === "xls") return "bg-emerald-50 text-emerald-700 border-emerald-200";
    if (e === "zip") return "bg-purple-50 text-purple-700 border-purple-200";
    return "bg-slate-50 text-slate-700 border-slate-200";
  };

  return (
    <div className="space-y-6">
      {/* Header Banner */}
      <div className="bg-gradient-to-r from-blue-900 via-indigo-900 to-slate-900 rounded-3xl p-8 text-white shadow-xl relative overflow-hidden">
        <div className="relative z-10 max-w-2xl">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-white/10 backdrop-blur-md text-xs font-semibold text-blue-200 mb-3 border border-white/10">
            <Globe className="w-3.5 h-3.5" />
            <span>Katalog Terverifikasi Publik</span>
          </div>
          <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
            Explorer Berkas Publik FITE
          </h2>
          <p className="text-sm text-blue-100 mt-2 leading-relaxed">
            Akses dokumen resmi, pedoman akademik, modul perkuliahan, dan berkas fakultas yang telah
            diverifikasi dan disetujui untuk publikasi terbuka bagi seluruh sivitas akademika.
          </p>
        </div>
      </div>

      {/* Filter & Search Bar */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-sm space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
          <div className="md:col-span-2 relative">
            <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Cari berkas publik berdasarkan judul atau kata kunci..."
              className="w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
            />
          </div>

          <div>
            <select
              value={prodi}
              onChange={(e) => setProdi(e.target.value)}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
            >
              <option value="">Semua Program Studi</option>
              <option value="IF">Informatika (IF)</option>
              <option value="SI">Sistem Informasi (SI)</option>
              <option value="TE">Teknik Elektro (TE)</option>
            </select>
          </div>

          <div>
            <select
              value={categoryId}
              onChange={(e) => setCategoryId(e.target.value)}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
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
      </div>

      {/* Document Grid */}
      {loading ? (
        <div className="p-16 text-center">
          <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm font-medium text-slate-500">Memuat berkas publik...</p>
        </div>
      ) : documents.length === 0 ? (
        <div className="bg-white rounded-2xl border border-slate-200/80 p-16 text-center shadow-sm">
          <FolderOpen className="w-14 h-14 text-slate-300 mx-auto mb-3" />
          <h4 className="text-base font-semibold text-slate-700">Tidak ada berkas publik yang ditemukan</h4>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
            Belum ada dokumen publik terverifikasi yang cocok dengan kriteria pencarian Anda.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {documents.map((doc) => (
            <div
              key={doc.id}
              className="bg-white rounded-2xl border border-slate-200/80 p-5 shadow-sm hover:shadow-md transition-all flex flex-col justify-between"
            >
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <span
                    className={`px-2.5 py-0.5 rounded-md text-xs font-bold uppercase tracking-wider border ${getExtensionColor(
                      doc.extension
                    )}`}
                  >
                    .{doc.extension}
                  </span>
                  <span className="text-xs font-medium text-slate-400">{formatSize(doc.size_kb)}</span>
                </div>

                <div>
                  <Link
                    href={`/berkas/${doc.id}`}
                    className="font-bold text-slate-900 hover:text-indigo-600 line-clamp-2 transition-colors"
                  >
                    {doc.title}
                  </Link>
                  <p className="text-xs text-slate-400 font-mono mt-1 line-clamp-1">
                    {doc.original_name}
                  </p>
                </div>

                {doc.description && (
                  <p className="text-xs text-slate-500 line-clamp-2">{doc.description}</p>
                )}

                <div className="flex flex-wrap items-center gap-2 pt-2 border-t border-slate-100 text-[11px] text-slate-500">
                  <span className="px-2 py-0.5 rounded bg-slate-100 font-medium">
                    {doc.category?.name || "Umum"}
                  </span>
                  {doc.prodi && (
                    <span className="px-2 py-0.5 rounded bg-blue-50 text-blue-700 font-medium">
                      {doc.prodi.code}
                    </span>
                  )}
                  <span className="ml-auto text-slate-400">
                    {new Date(doc.created_at).toLocaleDateString("id-ID")}
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-2 mt-5 pt-3 border-t border-slate-100">
                <button
                  onClick={() => handleDownload(doc)}
                  className="flex-1 inline-flex items-center justify-center gap-1.5 py-2 px-3 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-xs transition-colors"
                >
                  <Download className="w-3.5 h-3.5" />
                  <span>Unduh</span>
                </button>
                <Link
                  href={`/berkas/${doc.id}`}
                  className="p-2 rounded-xl border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
                  title="Lihat Detail"
                >
                  <ExternalLink className="w-4 h-4" />
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
