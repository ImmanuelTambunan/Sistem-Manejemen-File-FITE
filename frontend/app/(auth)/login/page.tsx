"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth";
import { Lock, Mail, Eye, EyeOff, AlertCircle, Clock, ShieldCheck, Loader2 } from "lucide-react";

function LoginForm() {
  const { user, login } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [rateLimitSeconds, setRateLimitSeconds] = useState<number | null>(null);

  useEffect(() => {
    if (user) {
      if (user.must_change_password) {
        router.push("/ganti-sandi");
      } else {
        router.push("/dashboard");
      }
    }
  }, [user, router]);

  useEffect(() => {
    if (searchParams.get("expired") === "1") {
      setErrorMessage("Sesi Anda telah berakhir. Silakan masuk kembali.");
    }
  }, [searchParams]);

  // Countdown timer for 429 Rate Limit
  useEffect(() => {
    if (rateLimitSeconds === null || rateLimitSeconds <= 0) return;
    const interval = setInterval(() => {
      setRateLimitSeconds((prev) => {
        if (prev === null || prev <= 1) {
          clearInterval(interval);
          setErrorMessage(null);
          return null;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [rateLimitSeconds]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setFieldErrors({});

    if (rateLimitSeconds !== null && rateLimitSeconds > 0) {
      return;
    }

    setLoading(true);
    try {
      const loggedUser = await login(email, password);
      if (loggedUser.must_change_password) {
        router.push("/ganti-sandi");
      } else {
        router.push("/dashboard");
      }
    } catch (err: any) {
      if (err.status === 429) {
        const retryHeader = err.retryAfter;
        const seconds = retryHeader ? parseInt(retryHeader, 10) : 60;
        setRateLimitSeconds(isNaN(seconds) ? 60 : seconds);
        setErrorMessage("Terlalu banyak percobaan masuk yang gagal. Mohon tunggu sebelum mencoba lagi.");
      } else if (err.status === 422 && err.errors) {
        setFieldErrors(err.errors);
        setErrorMessage(err.message || "Isian formulir tidak valid.");
      } else {
        setErrorMessage(err.message || "Gagal masuk. Periksa kembali email dan kata sandi Anda.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col justify-center py-12 sm:px-6 lg:px-8">
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        <div className="mx-auto w-12 h-12 rounded-2xl bg-blue-600 flex items-center justify-center text-white shadow-lg shadow-blue-500/30 mb-4">
          <ShieldCheck className="w-7 h-7" />
        </div>
        <h2 className="text-2xl font-bold tracking-tight text-slate-900">
          SIFITE
        </h2>
        <p className="mt-1 text-sm text-slate-500">
          Sistem Informasi Arsip Berkas Digital FITE
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md bg-white py-8 px-6 sm:px-10 shadow-xl shadow-slate-200/50 sm:rounded-2xl border border-slate-100">
        <form onSubmit={handleSubmit} className="space-y-5">
          {errorMessage && (
            <div
              className={`p-4 rounded-xl flex items-start gap-3 text-sm ${
                rateLimitSeconds !== null && rateLimitSeconds > 0
                  ? "bg-amber-50 text-amber-800 border border-amber-200"
                  : "bg-red-50 text-red-800 border border-red-200"
              }`}
            >
              {rateLimitSeconds !== null && rateLimitSeconds > 0 ? (
                <Clock className="w-5 h-5 flex-shrink-0 mt-0.5" />
              ) : (
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
              )}
              <div className="flex-1">
                <p className="font-medium">{errorMessage}</p>
                {rateLimitSeconds !== null && rateLimitSeconds > 0 && (
                  <p className="text-xs font-semibold mt-1">
                    Dapat mencoba kembali dalam: {rateLimitSeconds} detik
                  </p>
                )}
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-2">
              Email Institusi FITE
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
                <Mail className="w-5 h-5" />
              </div>
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="nama@del.ac.id"
                disabled={loading || (rateLimitSeconds !== null && rateLimitSeconds > 0)}
                className={`w-full pl-11 pr-4 py-3 rounded-xl border ${
                  fieldErrors.email ? "border-red-400 focus:ring-red-400" : "border-slate-200 focus:ring-blue-500"
                } focus:outline-none focus:ring-2 focus:border-transparent text-sm transition-all`}
              />
            </div>
            {fieldErrors.email && (
              <p className="text-xs text-red-600 mt-1.5">{fieldErrors.email[0]}</p>
            )}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-2">
              Kata Sandi
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
                <Lock className="w-5 h-5" />
              </div>
              <input
                type={showPassword ? "text" : "password"}
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                disabled={loading || (rateLimitSeconds !== null && rateLimitSeconds > 0)}
                className={`w-full pl-11 pr-11 py-3 rounded-xl border ${
                  fieldErrors.password ? "border-red-400 focus:ring-red-400" : "border-slate-200 focus:ring-blue-500"
                } focus:outline-none focus:ring-2 focus:border-transparent text-sm transition-all`}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute inset-y-0 right-0 pr-3.5 flex items-center text-slate-400 hover:text-slate-600"
              >
                {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
              </button>
            </div>
            {fieldErrors.password && (
              <p className="text-xs text-red-600 mt-1.5">{fieldErrors.password[0]}</p>
            )}
          </div>

          <button
            type="submit"
            disabled={loading || (rateLimitSeconds !== null && rateLimitSeconds > 0)}
            className="w-full py-3.5 px-4 rounded-xl bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white font-medium text-sm transition-colors shadow-md shadow-blue-500/20 flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Memproses...</span>
              </>
            ) : rateLimitSeconds && rateLimitSeconds > 0 ? (
              <span>Coba lagi dalam {rateLimitSeconds}s</span>
            ) : (
              <span>Masuk ke Akun</span>
            )}
          </button>
        </form>

        <div className="mt-8 pt-6 border-t border-slate-100 flex items-center justify-between text-xs text-slate-400">
          <span>Sistem Repositori Arsip FITE</span>
          <Link href="/status" className="text-slate-500 hover:text-blue-600 transition-colors font-medium">
            Status Layanan
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen bg-slate-50 flex items-center justify-center">
          <div className="w-8 h-8 border-4 border-blue-600 border-t-transparent rounded-full animate-spin" />
        </div>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
