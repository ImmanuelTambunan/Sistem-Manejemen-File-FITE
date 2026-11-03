"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  Bell,
  CheckCircle,
  XCircle,
  AlertCircle,
  CheckCheck,
  Trash2,
  ExternalLink,
  ChevronLeft,
  ChevronRight,
  Clock,
  Inbox,
} from "lucide-react";
import { api } from "@/lib/api";

interface NotificationItem {
  id: string;
  type: string;
  title: string;
  message: string;
  data?: Record<string, any>;
  is_read: boolean;
  created_at: string;
}

interface Meta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export default function NotificationPage() {
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 20, total: 0, total_pages: 1 });
  const [filterRead, setFilterRead] = useState<string>("all"); // 'all' | 'unread'
  const [loading, setLoading] = useState(true);
  const [markingAll, setMarkingAll] = useState(false);

  const fetchNotifications = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (filterRead === "unread") {
        params.append("is_read", "false");
      }
      params.append("page", String(meta.page));
      params.append("per_page", "20");

      const res = await api.get<NotificationItem[]>(
        `/api/v1/notifications?${params.toString()}`
      );
      setNotifications(res.data || []);
      if (res.meta) setMeta({
        page: res.meta.page ?? res.meta.current_page ?? 1,
        per_page: res.meta.per_page ?? 20,
        total: res.meta.total ?? 0,
        total_pages: res.meta.total_pages ?? res.meta.last_page ?? 1,
      });
    } catch (err) {
      console.error("Gagal memuat notifikasi:", err);
    } finally {
      setLoading(false);
    }
  }, [filterRead, meta.page]);

  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  const handleMarkAsRead = async (id: string) => {
    try {
      await api.patch(`/api/v1/notifications/${id}/read`);
      setNotifications((prev) =>
        prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
      );
    } catch (err) {
      console.error("Gagal menandai dibaca:", err);
    }
  };

  const handleMarkAllRead = async () => {
    setMarkingAll(true);
    try {
      await api.patch("/api/v1/notifications/read-all");
      setNotifications((prev) => prev.map((n) => ({ ...n, is_read: true })));
    } catch (err) {
      console.error("Gagal menandai semua dibaca:", err);
    } finally {
      setMarkingAll(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.delete(`/api/v1/notifications/${id}`);
      setNotifications((prev) => prev.filter((n) => n.id !== id));
    } catch (err) {
      console.error("Gagal menghapus notifikasi:", err);
    }
  };

  const getIcon = (type: string) => {
    if (type.includes("approved")) {
      return (
        <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-600 flex items-center justify-center flex-shrink-0">
          <CheckCircle className="w-5 h-5" />
        </div>
      );
    }
    if (type.includes("rejected")) {
      return (
        <div className="w-10 h-10 rounded-xl bg-red-100 text-red-600 flex items-center justify-center flex-shrink-0">
          <XCircle className="w-5 h-5" />
        </div>
      );
    }
    if (type.includes("retention") || type.includes("warning")) {
      return (
        <div className="w-10 h-10 rounded-xl bg-amber-100 text-amber-600 flex items-center justify-center flex-shrink-0">
          <AlertCircle className="w-5 h-5" />
        </div>
      );
    }
    return (
      <div className="w-10 h-10 rounded-xl bg-indigo-100 text-indigo-600 flex items-center justify-center flex-shrink-0">
        <Bell className="w-5 h-5" />
      </div>
    );
  };

  return (
    <div className="space-y-6 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Pusat Notifikasi</h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Pemberitahuan verifikasi, status dokumen, dan pembaruan sistem
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleMarkAllRead}
            disabled={markingAll || notifications.length === 0}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 text-xs font-semibold shadow-sm transition-colors disabled:opacity-50"
          >
            <CheckCheck className="w-4 h-4 text-indigo-600" />
            <span>Tandai Semua Dibaca</span>
          </button>
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="flex items-center gap-2 border-b border-slate-200 pb-2 text-xs">
        <button
          onClick={() => {
            setFilterRead("all");
            setMeta((m) => ({ ...m, page: 1 }));
          }}
          className={`px-3 py-1.5 rounded-lg font-semibold transition-colors ${
            filterRead === "all"
              ? "bg-indigo-600 text-white"
              : "text-slate-600 hover:bg-slate-100"
          }`}
        >
          Semua Notifikasi
        </button>
        <button
          onClick={() => {
            setFilterRead("unread");
            setMeta((m) => ({ ...m, page: 1 }));
          }}
          className={`px-3 py-1.5 rounded-lg font-semibold transition-colors ${
            filterRead === "unread"
              ? "bg-indigo-600 text-white"
              : "text-slate-600 hover:bg-slate-100"
          }`}
        >
          Belum Dibaca
        </button>
      </div>

      {/* Notifications List */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden divide-y divide-slate-100">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat pemberitahuan...</p>
          </div>
        ) : notifications.length === 0 ? (
          <div className="p-16 text-center">
            <Inbox className="w-14 h-14 text-slate-300 mx-auto mb-3" />
            <h4 className="text-base font-semibold text-slate-700">Tidak ada notifikasi</h4>
            <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
              Anda telah membaca semua pemberitahuan saat ini.
            </p>
          </div>
        ) : (
          notifications.map((item) => (
            <div
              key={item.id}
              className={`p-5 flex items-start gap-4 transition-colors ${
                !item.is_read ? "bg-indigo-50/30" : "hover:bg-slate-50/50"
              }`}
            >
              {getIcon(item.type)}

              <div className="flex-1 space-y-1">
                <div className="flex items-center justify-between">
                  <h4 className={`text-sm ${!item.is_read ? "font-bold text-slate-900" : "font-semibold text-slate-800"}`}>
                    {item.title}
                  </h4>
                  <span className="text-[11px] text-slate-400 flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {new Date(item.created_at).toLocaleString("id-ID", {
                      dateStyle: "medium",
                      timeStyle: "short",
                    })}
                  </span>
                </div>

                <p className="text-xs text-slate-600 leading-relaxed">{item.message}</p>

                {/* Additional link or action */}
                <div className="flex items-center gap-3 pt-2">
                  {item.data?.document_id && (
                    <Link
                      href={`/berkas/${item.data.document_id}`}
                      className="inline-flex items-center gap-1 text-xs font-semibold text-indigo-600 hover:text-indigo-800"
                    >
                      <span>Lihat Berkas</span>
                      <ExternalLink className="w-3.5 h-3.5" />
                    </Link>
                  )}

                  {!item.is_read && (
                    <button
                      onClick={() => handleMarkAsRead(item.id)}
                      className="text-xs font-medium text-slate-500 hover:text-slate-800"
                    >
                      Tandai dibaca
                    </button>
                  )}

                  <button
                    onClick={() => handleDelete(item.id)}
                    className="text-xs font-medium text-slate-400 hover:text-red-600 ml-auto flex items-center gap-1"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                    <span>Hapus</span>
                  </button>
                </div>
              </div>
            </div>
          ))
        )}

        {/* Pagination */}
        {meta.total_pages > 1 && (
          <div className="flex items-center justify-between px-6 py-4 bg-slate-50/50 text-xs">
            <span className="text-slate-500">
              Halaman {meta.page} dari {meta.total_pages}
            </span>
            <div className="flex items-center gap-1.5">
              <button
                disabled={meta.page <= 1}
                onClick={() => setMeta((m) => ({ ...m, page: Math.max(m.page - 1, 1) }))}
                className="p-1.5 rounded-lg border border-slate-200 text-slate-600 hover:bg-white disabled:opacity-40"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={meta.page >= meta.total_pages}
                onClick={() => setMeta((m) => ({ ...m, page: Math.min(m.page + 1, m.total_pages) }))}
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
