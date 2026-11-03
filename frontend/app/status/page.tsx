"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  CheckCircle2,
  AlertCircle,
  AlertTriangle,
  Server,
  Database,
  HardDrive,
  ShieldCheck,
  Clock,
  RefreshCw,
  ArrowLeft,
  Activity,
  Globe,
  LogIn,
} from "lucide-react";
import { getApiUrl } from "@/lib/api";

interface HealthCheckResult {
  status: "ok" | "error" | "loading";
  latencyMs: number;
  message?: string;
}

interface ComponentStatus {
  name: string;
  description: string;
  category: "core" | "storage" | "database" | "security" | "worker";
  status: "operational" | "degraded" | "outage" | "loading";
  latencyMs?: number;
  details?: string;
  icon: React.ReactNode;
}

export default function SystemStatusPage() {
  const [loading, setLoading] = useState(true);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [healthStatus, setHealthStatus] = useState<HealthCheckResult>({
    status: "loading",
    latencyMs: 0,
  });
  const [readyStatus, setReadyStatus] = useState<HealthCheckResult>({
    status: "loading",
    latencyMs: 0,
  });

  const checkStatus = useCallback(async () => {
    setLoading(true);

    // 1. Check /healthz (Core API Gateway)
    const t0Health = performance.now();
    let healthRes: HealthCheckResult = { status: "error", latencyMs: 0 };
    try {
      const res = await fetch(getApiUrl("/healthz"), {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      const t1Health = performance.now();
      const latency = Math.round(t1Health - t0Health);
      if (res.ok) {
        healthRes = { status: "ok", latencyMs: latency, message: "API Gateway responsif" };
      } else {
        healthRes = { status: "error", latencyMs: latency, message: `HTTP ${res.status}` };
      }
    } catch (err: any) {
      healthRes = {
        status: "error",
        latencyMs: 0,
        message: err.message || "Tidak dapat terhubung ke server",
      };
    }

    // 2. Check /readyz (Database & Storage Ping)
    const t0Ready = performance.now();
    let readyRes: HealthCheckResult = { status: "error", latencyMs: 0 };
    try {
      const res = await fetch(getApiUrl("/readyz"), {
        method: "GET",
        headers: { Accept: "application/json" },
        cache: "no-store",
      });
      const t1Ready = performance.now();
      const latency = Math.round(t1Ready - t0Ready);
      if (res.ok) {
        readyRes = {
          status: "ok",
          latencyMs: latency,
          message: "Database dan Storage terhubung",
        };
      } else {
        readyRes = { status: "error", latencyMs: latency, message: `HTTP ${res.status}` };
      }
    } catch (err: any) {
      readyRes = {
        status: "error",
        latencyMs: 0,
        message: err.message || "Koneksi dependensi gagal",
      };
    }

    setHealthStatus(healthRes);
    setReadyStatus(readyRes);
    setLastChecked(new Date());
    setLoading(false);
  }, []);

  useEffect(() => {
    checkStatus();
    // Auto-poll status every 30 seconds
    const interval = setInterval(checkStatus, 30000);
    return () => clearInterval(interval);
  }, [checkStatus]);

  const isAllOperational =
    healthStatus.status === "ok" && readyStatus.status === "ok";

  const components: ComponentStatus[] = [
    {
      name: "Core API Gateway",
      description: "Layanan REST API gin-gonic, CORS, rate limiter, dan middleware pipeline",
      category: "core",
      status: healthStatus.status === "ok" ? "operational" : "outage",
      latencyMs: healthStatus.latencyMs,
      details: healthStatus.message,
      icon: <Server className="w-5 h-5 text-indigo-600" />,
    },
    {
      name: "Database Cluster (MySQL 8.0)",
      description: "Penyimpanan data relasional berkas, pengguna, peran, dan riwayat audit",
      category: "database",
      status: readyStatus.status === "ok" ? "operational" : "outage",
      latencyMs: readyStatus.latencyMs,
      details: readyStatus.status === "ok" ? "Koneksi pool aktif & responsif" : readyStatus.message,
      icon: <Database className="w-5 h-5 text-blue-600" />,
    },
    {
      name: "Cloud Storage Engine (GCS)",
      description: "Repositori berkas digital terenkripsi dengan adapter Google Cloud Storage",
      category: "storage",
      status: readyStatus.status === "ok" ? "operational" : "degraded",
      latencyMs: readyStatus.latencyMs,
      details: readyStatus.status === "ok" ? "Bucket storage terhubung & terverifikasi" : readyStatus.message,
      icon: <HardDrive className="w-5 h-5 text-emerald-600" />,
    },
    {
      name: "Autentikasi & Keamanan Sesi",
      description: "Manajemen token JWT (15m), refresh rotation (7d), HttpOnly cookie, dan Bcrypt cost 12",
      category: "security",
      status: healthStatus.status === "ok" ? "operational" : "outage",
      details: "Perlindungan brute-force & reuse detection aktif",
      icon: <ShieldCheck className="w-5 h-5 text-purple-600" />,
    },
    {
      name: "Worker Retensi Dokumen",
      description: "Jadwal cron pembersihan berkas kedaluwarsa (02:00 WIB) dengan MySQL GET_LOCK",
      category: "worker",
      status: readyStatus.status === "ok" ? "operational" : "degraded",
      details: "Cut-off retensi 8 tahun & masa tenggang 90 hari aktif",
      icon: <Clock className="w-5 h-5 text-amber-600" />,
    },
  ];

  return (
    <div className="min-h-screen bg-slate-50 text-slate-800">
      {/* Top Navigation Bar */}
      <header className="bg-white border-b border-slate-200 sticky top-0 z-30 shadow-sm">
        <div className="max-w-5xl mx-auto px-4 sm:px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link
              href="/"
              className="flex items-center gap-2 text-slate-600 hover:text-slate-900 transition-colors text-sm font-medium mr-2"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Beranda</span>
            </Link>
            <div className="h-4 w-px bg-slate-300" />
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-indigo-600 flex items-center justify-center text-white font-bold text-sm shadow-sm">
                S
              </div>
              <div>
                <h1 className="text-sm font-bold text-slate-900 leading-tight">
                  SIFITE Status
                </h1>
                <p className="text-[11px] text-slate-500">
                  Fakultas Informatika & Teknik Elektro IT Del
                </p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <Link
              href="/publik"
              className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-200 text-xs font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              <Globe className="w-3.5 h-3.5" />
              <span>Explorer Publik</span>
            </Link>
            <Link
              href="/login"
              className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-indigo-600 text-white text-xs font-semibold hover:bg-indigo-700 transition-colors shadow-sm"
            >
              <LogIn className="w-3.5 h-3.5" />
              <span>Masuk</span>
            </Link>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-5xl mx-auto px-4 sm:px-6 py-8 space-y-8">
        {/* Overall Status Banner */}
        <div
          className={`rounded-2xl p-6 sm:p-8 border shadow-sm transition-all ${
            loading
              ? "bg-slate-100 border-slate-200"
              : isAllOperational
              ? "bg-emerald-50/70 border-emerald-200"
              : "bg-red-50/70 border-red-200"
          }`}
        >
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-start sm:items-center gap-4">
              <div
                className={`w-14 h-14 rounded-2xl flex items-center justify-center flex-shrink-0 ${
                  loading
                    ? "bg-slate-200 text-slate-500"
                    : isAllOperational
                    ? "bg-emerald-500 text-white shadow-lg shadow-emerald-500/20"
                    : "bg-red-500 text-white shadow-lg shadow-red-500/20"
                }`}
              >
                {loading ? (
                  <RefreshCw className="w-7 h-7 animate-spin" />
                ) : isAllOperational ? (
                  <CheckCircle2 className="w-8 h-8" />
                ) : (
                  <AlertCircle className="w-8 h-8" />
                )}
              </div>
              <div>
                <h2 className="text-xl sm:text-2xl font-bold text-slate-900">
                  {loading
                    ? "Memeriksa Status Sistem..."
                    : isAllOperational
                    ? "Semua Layanan Beroperasi Normal"
                    : "Sebagian Layanan Mengalami Gangguan"}
                </h2>
                <p className="text-sm text-slate-600 mt-1">
                  {loading
                    ? "Menghubungi endpoint healthz dan readyz backend..."
                    : isAllOperational
                    ? "Seluruh komponen sistem arsip digital FITE berfungsi optimal tanpa kendala."
                    : "Terjadi kegagalan komunikasi pada komponen backend atau penyimpanan."}
                </p>
              </div>
            </div>

            <button
              onClick={() => checkStatus()}
              disabled={loading}
              className="inline-flex items-center justify-center gap-2 px-4 py-2 rounded-xl bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 text-xs font-semibold shadow-sm transition-colors self-start sm:self-auto disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} />
              <span>Periksa Sekarang</span>
            </button>
          </div>

          {lastChecked && (
            <div className="mt-4 pt-4 border-t border-slate-200/60 flex items-center justify-between text-xs text-slate-500">
              <span className="flex items-center gap-1.5">
                <Clock className="w-3.5 h-3.5" />
                Terakhir diperiksa: {lastChecked.toLocaleTimeString("id-ID")} WIB
              </span>
              <span className="font-mono text-[11px] bg-white/70 px-2 py-0.5 rounded border border-slate-200">
                Latensi API: {healthStatus.latencyMs}ms
              </span>
            </div>
          )}
        </div>

        {/* System Components Breakdown */}
        <div className="bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between">
            <div>
              <h3 className="font-bold text-slate-900 text-base">Komponen Layanan SIFITE</h3>
              <p className="text-xs text-slate-500 mt-0.5">
                Pemantauan berkala kesehatan subsistem dan infrastruktur
              </p>
            </div>
            <div className="flex items-center gap-2 text-xs font-medium text-slate-500">
              <Activity className="w-4 h-4 text-indigo-500" />
              <span>Live Monitor</span>
            </div>
          </div>

          <div className="divide-y divide-slate-100">
            {components.map((comp) => {
              const isOk = comp.status === "operational";
              return (
                <div
                  key={comp.name}
                  className="p-5 sm:p-6 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:bg-slate-50/60 transition-colors"
                >
                  <div className="flex items-start gap-4">
                    <div className="p-2.5 rounded-xl bg-slate-100 border border-slate-200/60 flex-shrink-0 mt-0.5">
                      {comp.icon}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <h4 className="font-bold text-slate-800 text-sm">{comp.name}</h4>
                        {comp.latencyMs !== undefined && comp.latencyMs > 0 && (
                          <span className="font-mono text-[10px] text-slate-400 bg-slate-100 px-1.5 py-0.5 rounded">
                            {comp.latencyMs} ms
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-slate-500 mt-1 max-w-xl">
                        {comp.description}
                      </p>
                      {comp.details && (
                        <p className="text-[11px] text-slate-400 mt-1 italic">
                          Catatan: {comp.details}
                        </p>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-2 self-start sm:self-center">
                    <span
                      className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold ${
                        isOk
                          ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                          : "bg-red-50 text-red-700 border border-red-200"
                      }`}
                    >
                      <span
                        className={`w-1.5 h-1.5 rounded-full ${
                          isOk ? "bg-emerald-500 animate-pulse" : "bg-red-500"
                        }`}
                      />
                      {isOk ? "Operasional" : "Gangguan"}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* System Specifications & Architecture Metrics */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">
              Versi Rilis
            </span>
            <div className="flex items-baseline gap-2">
              <span className="text-xl font-bold text-slate-800 font-mono">v1.0.0-FITE</span>
              <span className="text-xs text-emerald-600 font-medium">Stabil</span>
            </div>
            <p className="text-xs text-slate-500 mt-2">
              Arsitektur terpisah Go Backend + Next.js Frontend.
            </p>
          </div>

          <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">
              Protokol Autentikasi
            </span>
            <div className="flex items-baseline gap-2">
              <span className="text-xl font-bold text-slate-800 font-mono">JWT + HttpOnly</span>
            </div>
            <p className="text-xs text-slate-500 mt-2">
              Access Token 15 menit, Refresh Token 7 hari dengan deteksi reuse.
            </p>
          </div>

          <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm">
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-1">
              Zona Waktu Operasional
            </span>
            <div className="flex items-baseline gap-2">
              <span className="text-xl font-bold text-slate-800 font-mono">WIB (UTC+7)</span>
              <span className="text-xs text-slate-500">Asia/Jakarta</span>
            </div>
            <p className="text-xs text-slate-500 mt-2">
              Worker retensi dieksekusi setiap pukul 02:00 WIB.
            </p>
          </div>
        </div>

        {/* Footer */}
        <div className="pt-6 border-t border-slate-200 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-500">
          <p>© 2026 Institut Teknologi Del — Fakultas Informatika dan Teknik Elektro.</p>
          <div className="flex items-center gap-4">
            <Link href="/publik" className="hover:text-slate-800 transition-colors">
              Explorer Publik
            </Link>
            <Link href="/login" className="hover:text-slate-800 transition-colors">
              Masuk Sistem
            </Link>
            <a
              href="mailto:helpdesk@del.ac.id"
              className="hover:text-slate-800 transition-colors"
            >
              Bantuan TI
            </a>
          </div>
        </div>
      </main>
    </div>
  );
}
