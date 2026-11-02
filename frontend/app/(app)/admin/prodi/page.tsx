"use client";

import React, { useState, useEffect } from "react";
import {
  GraduationCap,
  Edit2,
  Building,
  CheckCircle,
  AlertCircle,
  Users,
} from "lucide-react";
import { api } from "@/lib/api";

interface Prodi {
  id: number;
  name: string;
  code: string;
}

export default function ProdiManagementPage() {
  const [prodis, setProdis] = useState<Prodi[]>([]);
  const [loading, setLoading] = useState(true);

  // Edit Modal
  const [editTarget, setEditTarget] = useState<Prodi | null>(null);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const fetchProdis = async () => {
    setLoading(true);
    try {
      const res = await api.get<Prodi[]>("/api/v1/users/prodi");
      setProdis(res.data || []);
    } catch (err) {
      console.error("Gagal memuat prodi:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProdis();
  }, []);

  const openEditModal = (p: Prodi) => {
    setEditTarget(p);
    setName(p.name);
    setCode(p.code);
    setFormError(null);
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editTarget) return;

    setFormLoading(true);
    setFormError(null);
    try {
      await api.put(`/api/v1/users/prodi/${editTarget.id}`, {
        name: name.trim(),
        code: code.trim().toUpperCase(),
      });
      setEditTarget(null);
      fetchProdis();
    } catch (err: any) {
      setFormError(err.message || "Gagal memperbarui data Program Studi.");
    } finally {
      setFormLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Program Studi FITE</h2>
        <p className="text-xs text-slate-500 mt-0.5">
          Daftar program studi resmi di bawah naungan Fakultas Informatika & Teknik Elektro
        </p>
      </div>

      {/* Grid Cards of Prodi */}
      {loading ? (
        <div className="p-16 text-center">
          <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm font-medium text-slate-500">Memuat program studi...</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          {prodis.map((p) => (
            <div
              key={p.id}
              className="bg-white rounded-2xl border border-slate-200/80 p-6 shadow-sm flex flex-col justify-between"
            >
              <div>
                <div className="flex items-center justify-between mb-4">
                  <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center font-bold text-base">
                    {p.code}
                  </div>
                  <button
                    onClick={() => openEditModal(p)}
                    className="p-2 text-slate-400 hover:text-indigo-600 hover:bg-slate-50 rounded-lg transition-colors"
                    title="Ubah Nama/Kode"
                  >
                    <Edit2 className="w-4 h-4" />
                  </button>
                </div>

                <h3 className="text-lg font-bold text-slate-900">{p.name}</h3>
                <p className="text-xs font-mono text-indigo-600 font-semibold mt-0.5">Kode: {p.code}</p>

                <div className="mt-4 pt-4 border-t border-slate-100 space-y-2 text-xs text-slate-500">
                  <div className="flex justify-between">
                    <span>Jenjang:</span>
                    <span className="font-semibold text-slate-800">Sarjana (S1)</span>
                  </div>
                  <div className="flex justify-between">
                    <span>Fakultas:</span>
                    <span className="font-semibold text-slate-800">Fakultas Informatika & Elektro</span>
                  </div>
                </div>
              </div>

              <div className="mt-6 pt-3 border-t border-slate-100 flex items-center justify-between text-xs">
                <span className="inline-flex items-center gap-1 text-emerald-600 font-medium">
                  <CheckCircle className="w-3.5 h-3.5" /> Terdaftar Aktif
                </span>
                <span className="text-slate-400 font-mono">ID: #{p.id}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Edit Modal */}
      {editTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <h3 className="text-lg font-bold text-slate-900">Ubah Program Studi</h3>
            <p className="text-xs text-slate-500 mt-0.5">Perbarui nama atau singkatan kode program studi.</p>

            {formError && (
              <div className="mt-3 p-3 rounded-xl bg-red-50 border border-red-200 text-red-700 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleUpdate} className="mt-4 space-y-3.5">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Nama Program Studi <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm focus:ring-2 focus:ring-indigo-200"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Kode Prodi (Singkatan) <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm font-mono focus:ring-2 focus:ring-indigo-200 uppercase"
                  required
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setEditTarget(null)}
                  disabled={formLoading}
                  className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={formLoading}
                  className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold flex items-center gap-2"
                >
                  {formLoading ? "Menyimpan..." : "Simpan Perubahan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
