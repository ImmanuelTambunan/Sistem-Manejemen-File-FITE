"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  HardDrive,
  Clock,
  ShieldAlert,
  Search,
  Filter,
  FileText,
  User,
  ChevronLeft,
  ChevronRight,
  Database,
  Trash2,
  UploadCloud,
  Download,
  AlertTriangle,
} from "lucide-react";
import { api } from "@/lib/api";

interface StorageMetrics {
  total_documents: number;
  total_size_mb: number;
  expiring_within_90_days: number;
  soft_deleted_in_grace_period: number;
}

interface StorageLogItem {
  id: number;
  action: string;
  document_id?: number;
  user?: { id: number; name: string; role: string };
  meta?: Record<string, any>;
  created_at: string;
}

interface Meta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export default function StorageAuditLogPage() {
  const [metrics, setMetrics] = useState<StorageMetrics | null>(null);
  const [logs, setLogs] = useState<StorageLogItem[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 15, total: 0, total_pages: 1 });
  const [loading, setLoading] = useState(true);

  // Filters
  const [action, setAction] = useState("");
  const [documentId, setDocumentId] = useState("");
  const [page, setPage] = useState(1);

  const fetchStorageData = useCallback(async () => {
    setLoading(true);
    try {
      const [storageRes, logsRes] = await Promise.all([
        api.get<StorageMetrics>("/api/v1/dashboard/storage"),
        api.get<StorageLogItem[]>(
          `/api/v1/dashboard/storage-logs?${new URLSearchParams({
            ...(action ? { action } : {}),
            ...(documentId ? { document_id: documentId } : {}),
            page: String(page),
            per_page: "15",
          }).toString()}`
        ),
      ]);

      setMetrics(storageRes.data);
      setLogs(logsRes.data || []);
      if (logsRes.meta) setMeta({
        page: logsRes.meta.page ?? logsRes.meta.current_page ?? 1,
        per_page: logsRes.meta.per_page ?? 15,
        total: logsRes.meta.total ?? 0,
        total_pages: logsRes.meta.total_pages ?? logsRes.meta.last_page ?? 1,
      });
    } catch (err) {
      console.error("Gagal memuat log penyimpanan:", err);
    } finally {
      setLoading(false);
    }
  }, [action, documentId, page]);

  useEffect(() => {
    fetchStorageData();
  }, [fetchStorageData]);

  const getActionBadge = (act: string) => {
    switch (act.toLowerCase()) {
      case "upload":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-100 text-emerald-800">
            <UploadCloud className="w-3 h-3" /> UPLOAD
          </span>
        );
      case "download":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-100 text-blue-800">
            <Download className="w-3 h-3" /> DOWNLOAD
          </span>
        );
      case "soft_delete":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800">
            <Trash2 className="w-3 h-3" /> SOFT DELETE
          </span>
        );
      case "hard_delete":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-red-100 text-red-800">
            <Trash2 className="w-3 h-3" /> HARD DELETE
          </span>
        );
      case "retention_expired":
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-purple-100 text-purple-800">
            <Clock className="w-3 h-3" /> RETENTION EXPIRED
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-800 uppercase">
            {act}
          </span>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Audit Trail & Log Penyimpanan</h2>
        <p className="text-xs text-slate-500 mt-0.5">
          Pantau konsumsi penyimpanan cloud GCS, siklus retensi 8 tahun, dan jejak aktivitas berkas
        </p>
      </div>

      {/* Storage KPI Cards */}
      {metrics && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Total Dokumen Aktif</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{metrics.total_documents}</h4>
              <p className="text-xs text-slate-400 mt-1">Tersimpan di sistem</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
              <Database className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Kapasitas Terpakai</p>
              <h4 className="text-2xl font-bold text-indigo-600 mt-1">
                {metrics.total_size_mb} <span className="text-sm font-medium text-slate-400">MB</span>
              </h4>
              <p className="text-xs text-slate-400 mt-1">Cloud Storage (GCS)</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center">
              <HardDrive className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Masa Tenggang (90 Hari)</p>
              <h4 className="text-2xl font-bold text-amber-600 mt-1">{metrics.soft_deleted_in_grace_period}</h4>
              <p className="text-xs text-slate-400 mt-1">Menunggu pembersihan permanen</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center">
              <Clock className="w-6 h-6" />
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-sm flex items-center justify-between">
            <div>
              <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Retensi 8 Tahun Dekat</p>
              <h4 className="text-2xl font-bold text-slate-900 mt-1">{metrics.expiring_within_90_days}</h4>
              <p className="text-xs text-slate-400 mt-1">Kadaluarsa dalam &le; 90 hari</p>
            </div>
            <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center">
              <AlertTriangle className="w-6 h-6" />
            </div>
          </div>
        </div>
      )}

      {/* Filter Toolbar */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-sm flex flex-col sm:flex-row gap-3">
        <div className="w-full sm:w-64">
          <select
            value={action}
            onChange={(e) => {
              setAction(e.target.value);
              setPage(1);
            }}
            className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
          >
            <option value="">Semua Aksi</option>
            <option value="upload">UPLOAD</option>
            <option value="download">DOWNLOAD</option>
            <option value="update_metadata">UPDATE METADATA</option>
            <option value="soft_delete">SOFT DELETE</option>
            <option value="hard_delete">HARD DELETE</option>
            <option value="retention_expired">RETENTION EXPIRED</option>
            <option value="restore">RESTORE</option>
          </select>
        </div>

        <div className="w-full sm:w-48">
          <input
            type="number"
            value={documentId}
            onChange={(e) => {
              setDocumentId(e.target.value);
              setPage(1);
            }}
            placeholder="Cari ID Dokumen..."
            className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs focus:outline-none focus:ring-2 focus:ring-indigo-200"
          />
        </div>
      </div>

      {/* Logs Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat log penyimpanan...</p>
          </div>
        ) : logs.length === 0 ? (
          <div className="p-16 text-center">
            <Database className="w-14 h-14 text-slate-300 mx-auto mb-3" />
            <h4 className="text-base font-semibold text-slate-700">Belum ada catatan log</h4>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Waktu</th>
                  <th className="px-6 py-3.5">Aksi</th>
                  <th className="px-6 py-3.5">Dokumen Terkait</th>
                  <th className="px-6 py-3.5">Aktor (Pengguna)</th>
                  <th className="px-6 py-3.5">Metadata Tambahan</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-50/60 transition-colors">
                    <td className="px-6 py-4 text-xs font-mono text-slate-500 whitespace-nowrap">
                      {new Date(log.created_at).toLocaleString("id-ID", {
                        dateStyle: "short",
                        timeStyle: "medium",
                      })}
                    </td>
                    <td className="px-6 py-4">{getActionBadge(log.action)}</td>
                    <td className="px-6 py-4">
                      {log.document_id ? (
                        <Link
                          href={`/berkas/${log.document_id}`}
                          className="font-mono text-xs font-semibold text-indigo-600 hover:underline"
                        >
                          Dokumen #{log.document_id}
                        </Link>
                      ) : (
                        <span className="text-xs text-slate-400">-</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {log.user ? (
                        <div>
                          <p className="text-xs font-semibold text-slate-800">{log.user.name}</p>
                          <p className="text-[11px] text-slate-400 uppercase">{log.user.role}</p>
                        </div>
                      ) : (
                        <span className="text-xs text-slate-400 italic">Sistem Worker</span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      {log.meta && Object.keys(log.meta).length > 0 ? (
                        <pre className="text-[11px] bg-slate-50 p-2 rounded-lg font-mono text-slate-600 max-w-xs overflow-x-auto">
                          {JSON.stringify(log.meta)}
                        </pre>
                      ) : (
                        <span className="text-xs text-slate-400">-</span>
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
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 bg-slate-50/50 text-xs">
            <span className="text-slate-500">
              Halaman {meta.page} dari {meta.total_pages} ({meta.total} log)
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
    </div>
  );
}
