"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  CheckCircle,
  XCircle,
  Clock,
  Filter,
  Check,
  X,
  AlertCircle,
  ExternalLink,
  ChevronLeft,
  ChevronRight,
  ShieldCheck,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";

interface VerificationItem {
  id: number;
  document: { id: number; title: string; prodi?: string };
  status: string;
  reason?: string;
  requested_by: { id: number; name: string };
  verified_by?: { id: number; name: string };
  requested_at: string;
  verified_at?: string;
}

interface Meta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export default function VerificationPage() {
  const { user } = useAuth();
  const [items, setItems] = useState<VerificationItem[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 15, total: 0, total_pages: 1 });
  const [loading, setLoading] = useState(true);

  // Filters
  const [status, setStatus] = useState<string>("pending");
  const [prodi, setProdi] = useState<string>("");
  const [page, setPage] = useState<number>(1);

  // Action modals
  const [approveTarget, setApproveTarget] = useState<VerificationItem | null>(null);
  const [rejectTarget, setRejectTarget] = useState<VerificationItem | null>(null);
  const [rejectReason, setRejectReason] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const fetchVerifications = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (status) params.append("status", status);
      if (prodi) params.append("prodi", prodi);
      params.append("page", String(page));
      params.append("per_page", "15");

      const res = await api.get<VerificationItem[]>(
        `/api/v1/verifications?${params.toString()}`
      );
      setItems(res.data || []);
      if (res.meta) setMeta(res.meta);
    } catch (err) {
      console.error("Gagal memuat verifikasi:", err);
    } finally {
      setLoading(false);
    }
  }, [status, prodi, page]);

  useEffect(() => {
    fetchVerifications();
  }, [fetchVerifications]);

  const handleApprove = async () => {
    if (!approveTarget) return;
    setActionLoading(true);
    setActionError(null);
    try {
      await api.patch(`/api/v1/verifications/${approveTarget.id}`, {
        status: "approved",
      });
      setApproveTarget(null);
      fetchVerifications();
    } catch (err: any) {
      setActionError(err.message || "Gagal menyetujui berkas.");
    } finally {
      setActionLoading(false);
    }
  };

  const handleReject = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!rejectTarget) return;
    if (rejectReason.trim().length < 5 || rejectReason.trim().length > 500) {
      setActionError("Alasan penolakan wajib diisi antara 5 hingga 500 karakter.");
      return;
    }

    setActionLoading(true);
    setActionError(null);
    try {
      await api.patch(`/api/v1/verifications/${rejectTarget.id}`, {
        status: "rejected",
        reason: rejectReason.trim(),
      });
      setRejectTarget(null);
      setRejectReason("");
      fetchVerifications();
    } catch (err: any) {
      setActionError(err.message || "Gagal menolak berkas.");
    } finally {
      setActionLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-md bg-amber-50 text-amber-800 text-xs font-semibold mb-1">
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>Hak Akses Verifikator (BAAK & Super Admin)</span>
          </div>
          <h2 className="text-2xl font-bold text-slate-900 tracking-tight">
            Panel Verifikasi Berkas Publik
          </h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Tinjau dan putuskan permintaan publikasi dokumen fakultas dan program studi
          </p>
        </div>
      </div>

      {/* Status Tabs & Filters */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-sm flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        {/* Tabs */}
        <div className="flex items-center gap-2 overflow-x-auto pb-1 md:pb-0">
          <button
            onClick={() => {
              setStatus("pending");
              setPage(1);
            }}
            className={`px-4 py-2 rounded-xl text-xs font-semibold transition-colors flex items-center gap-2 ${
              status === "pending"
                ? "bg-amber-500 text-white shadow-sm"
                : "text-slate-600 hover:bg-slate-100"
            }`}
          >
            <Clock className="w-3.5 h-3.5" />
            <span>Menunggu Verifikasi</span>
          </button>

          <button
            onClick={() => {
              setStatus("approved");
              setPage(1);
            }}
            className={`px-4 py-2 rounded-xl text-xs font-semibold transition-colors flex items-center gap-2 ${
              status === "approved"
                ? "bg-emerald-600 text-white shadow-sm"
                : "text-slate-600 hover:bg-slate-100"
            }`}
          >
            <CheckCircle className="w-3.5 h-3.5" />
            <span>Disetujui</span>
          </button>

          <button
            onClick={() => {
              setStatus("rejected");
              setPage(1);
            }}
            className={`px-4 py-2 rounded-xl text-xs font-semibold transition-colors flex items-center gap-2 ${
              status === "rejected"
                ? "bg-red-600 text-white shadow-sm"
                : "text-slate-600 hover:bg-slate-100"
            }`}
          >
            <XCircle className="w-3.5 h-3.5" />
            <span>Ditolak</span>
          </button>

          <button
            onClick={() => {
              setStatus("");
              setPage(1);
            }}
            className={`px-4 py-2 rounded-xl text-xs font-semibold transition-colors ${
              status === ""
                ? "bg-slate-900 text-white shadow-sm"
                : "text-slate-600 hover:bg-slate-100"
            }`}
          >
            Semua Riwayat
          </button>
        </div>

        {/* Prodi Filter */}
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-slate-400 uppercase">Prodi:</span>
          <select
            value={prodi}
            onChange={(e) => {
              setProdi(e.target.value);
              setPage(1);
            }}
            className="px-3 py-1.5 rounded-lg border border-slate-200 text-xs bg-white text-slate-700 focus:outline-none focus:ring-2 focus:ring-indigo-200"
          >
            <option value="">Semua Prodi</option>
            <option value="IF">Informatika (IF)</option>
            <option value="SI">Sistem Informasi (SI)</option>
            <option value="TE">Teknik Elektro (TE)</option>
          </select>
        </div>
      </div>

      {/* Verification Items Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-amber-500 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat antrean verifikasi...</p>
          </div>
        ) : items.length === 0 ? (
          <div className="p-16 text-center">
            <CheckCircle className="w-14 h-14 text-slate-300 mx-auto mb-3" />
            <h4 className="text-base font-semibold text-slate-700">Tidak ada permohonan</h4>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              Tidak ada dokumen pada status &quot;{status || "semua"}&quot; yang perlu diproses saat ini.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Dokumen</th>
                  <th className="px-6 py-3.5">Prodi</th>
                  <th className="px-6 py-3.5">Pemohon</th>
                  <th className="px-6 py-3.5">Waktu Pengajuan</th>
                  <th className="px-6 py-3.5">Status</th>
                  <th className="px-6 py-3.5 text-right">Aksi Putusan</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {items.map((item) => (
                  <tr key={item.id} className="hover:bg-slate-50/60 transition-colors">
                    <td className="px-6 py-4">
                      <div>
                        <Link
                          href={`/berkas/${item.document.id}`}
                          className="font-semibold text-slate-900 hover:text-indigo-600 line-clamp-1 transition-colors"
                        >
                          {item.document.title}
                        </Link>
                        <span className="text-xs text-slate-400 font-mono">
                          Dokumen #{item.document.id}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs font-semibold text-slate-600">
                        {item.document.prodi || "Fakultas"}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-xs font-medium text-slate-800">
                        {item.requested_by.name}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-xs text-slate-500">
                      {new Date(item.requested_at).toLocaleString("id-ID", {
                        dateStyle: "medium",
                        timeStyle: "short",
                      })}
                    </td>
                    <td className="px-6 py-4">
                      {item.status === "pending" && (
                        <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800">
                          <Clock className="w-3 h-3" /> Menunggu
                        </span>
                      )}
                      {item.status === "approved" && (
                        <div>
                          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-100 text-emerald-800">
                            <Check className="w-3 h-3" /> Disetujui
                          </span>
                          {item.verified_by && (
                            <p className="text-[11px] text-slate-400 mt-0.5">
                              oleh {item.verified_by.name}
                            </p>
                          )}
                        </div>
                      )}
                      {item.status === "rejected" && (
                        <div>
                          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-100 text-red-800">
                            <X className="w-3 h-3" /> Ditolak
                          </span>
                          {item.reason && (
                            <p className="text-[11px] text-red-600 italic mt-0.5 max-w-xs">
                              &quot;{item.reason}&quot;
                            </p>
                          )}
                        </div>
                      )}
                    </td>
                    <td className="px-6 py-4 text-right">
                      {item.status === "pending" ? (
                        <div className="flex items-center justify-end gap-2">
                          <button
                            onClick={() => {
                              setActionError(null);
                              setApproveTarget(item);
                            }}
                            className="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-semibold shadow-sm transition-colors"
                          >
                            <Check className="w-3.5 h-3.5" />
                            <span>Setujui</span>
                          </button>
                          <button
                            onClick={() => {
                              setActionError(null);
                              setRejectReason("");
                              setRejectTarget(item);
                            }}
                            className="inline-flex items-center gap-1 px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-700 text-white text-xs font-semibold shadow-sm transition-colors"
                          >
                            <X className="w-3.5 h-3.5" />
                            <span>Tolak</span>
                          </button>
                          <Link
                            href={`/berkas/${item.document.id}`}
                            className="p-1.5 text-slate-400 hover:text-slate-600 rounded-lg"
                            title="Tinjau Berkas"
                          >
                            <ExternalLink className="w-4 h-4" />
                          </Link>
                        </div>
                      ) : (
                        <Link
                          href={`/berkas/${item.document.id}`}
                          className="inline-flex items-center gap-1 text-xs font-semibold text-indigo-600 hover:text-indigo-800"
                        >
                          <span>Rincian Berkas</span>
                          <ExternalLink className="w-3.5 h-3.5" />
                        </Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {meta.total_pages > 1 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 bg-slate-50/40 text-xs">
            <span className="text-slate-500">
              Halaman {meta.page} dari {meta.total_pages} ({meta.total} permohonan)
            </span>
            <div className="flex items-center gap-1.5">
              <button
                disabled={meta.page <= 1}
                onClick={() => setPage((p) => Math.max(p - 1, 1))}
                className="p-1.5 rounded-lg border border-slate-200 text-slate-600 hover:bg-white disabled:opacity-40"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={meta.page >= meta.total_pages}
                onClick={() => setPage((p) => Math.min(p + 1, meta.total_pages))}
                className="p-1.5 rounded-lg border border-slate-200 text-slate-600 hover:bg-white disabled:opacity-40"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Approve Confirmation Modal */}
      {approveTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center mb-4">
              <CheckCircle className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Setujui Publikasi Dokumen?</h3>
            <p className="text-xs text-slate-500 mt-1">
              Dokumen <strong className="text-slate-800">{approveTarget.document.title}</strong> akan
              segera dapat diakses oleh publik di katalog explorer publik.
            </p>

            {actionError && (
              <div className="mt-3 p-3 rounded-lg bg-red-50 text-red-700 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>{actionError}</span>
              </div>
            )}

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                onClick={() => setApproveTarget(null)}
                disabled={actionLoading}
                className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                onClick={handleApprove}
                disabled={actionLoading}
                className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-semibold shadow-sm flex items-center gap-2"
              >
                {actionLoading ? "Memproses..." : "Ya, Setujui"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Reject Modal with Reason Input */}
      {rejectTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center mb-4">
              <XCircle className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Tolak Permohonan Publikasi</h3>
            <p className="text-xs text-slate-500 mt-1">
              Dokumen: <strong className="text-slate-800">{rejectTarget.document.title}</strong>.
              Pengunggah akan menerima notifikasi beserta alasan penolakan ini.
            </p>

            <form onSubmit={handleReject} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Alasan Penolakan <span className="text-red-500">* (5 - 500 karakter)</span>
                </label>
                <textarea
                  rows={3}
                  value={rejectReason}
                  onChange={(e) => setRejectReason(e.target.value)}
                  placeholder="Contoh: Dokumen belum memiliki tanda tangan Kaprodi / format tidak sesuai pedoman..."
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm focus:outline-none focus:ring-2 focus:ring-red-200 focus:border-red-500"
                  required
                  minLength={5}
                  maxLength={500}
                />
                <span className="text-[11px] text-slate-400">{rejectReason.length}/500 karakter</span>
              </div>

              {actionError && (
                <div className="p-3 rounded-lg bg-red-50 text-red-700 text-xs flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 flex-shrink-0" />
                  <span>{actionError}</span>
                </div>
              )}

              <div className="flex items-center justify-end gap-3 pt-2 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setRejectTarget(null)}
                  disabled={actionLoading}
                  className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={actionLoading || rejectReason.trim().length < 5}
                  className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-xs font-semibold shadow-sm flex items-center gap-2 disabled:opacity-50"
                >
                  {actionLoading ? "Memproses..." : "Tolak Dokumen"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
