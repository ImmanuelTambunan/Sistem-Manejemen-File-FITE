"use client";

import React, { useState, useEffect } from "react";
import {
  FolderPlus,
  Edit2,
  Trash2,
  Check,
  AlertCircle,
  ShieldCheck,
  Tag,
  Layers,
} from "lucide-react";
import { api } from "@/lib/api";

interface Category {
  id: number;
  name: string;
  allowed_roles: string[];
  is_operational: boolean;
}

const ALL_ROLES = [
  { key: "super_admin", label: "Super Admin" },
  { key: "dekan", label: "Dekan" },
  { key: "baak", label: "BAAK" },
  { key: "kaprodi", label: "Kaprodi" },
  { key: "dosen", label: "Dosen" },
  { key: "mahasiswa", label: "Mahasiswa" },
];

export default function CategoryManagementPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);

  // Modal
  const [modalOpen, setModalOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [name, setName] = useState("");
  const [allowedRoles, setAllowedRoles] = useState<string[]>([]);
  const [isOperational, setIsOperational] = useState(false);
  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Delete
  const [deleteTarget, setDeleteTarget] = useState<Category | null>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchCategories = async () => {
    setLoading(true);
    try {
      const res = await api.get<Category[]>("/api/v1/documents/categories");
      setCategories(res.data || []);
    } catch (err) {
      console.error("Gagal memuat kategori:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCategories();
  }, []);

  const openCreateModal = () => {
    setEditingId(null);
    setName("");
    setAllowedRoles(["mahasiswa", "dosen", "kaprodi", "baak", "dekan", "super_admin"]);
    setIsOperational(false);
    setFormError(null);
    setModalOpen(true);
  };

  const openEditModal = (c: Category) => {
    setEditingId(c.id);
    setName(c.name);
    setAllowedRoles(c.allowed_roles || []);
    setIsOperational(c.is_operational || false);
    setFormError(null);
    setModalOpen(true);
  };

  const toggleRole = (roleKey: string) => {
    setAllowedRoles((prev) =>
      prev.includes(roleKey) ? prev.filter((r) => r !== roleKey) : [...prev, roleKey]
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setFormError("Nama kategori wajib diisi.");
      return;
    }
    if (allowedRoles.length === 0) {
      setFormError("Pilih minimal satu peran yang diizinkan mengunggah kategori ini.");
      return;
    }

    setFormLoading(true);
    setFormError(null);
    try {
      const body = {
        name: name.trim(),
        allowed_roles: allowedRoles,
        is_operational: isOperational,
      };

      if (editingId) {
        await api.put(`/api/v1/documents/categories/${editingId}`, body);
      } else {
        await api.post("/api/v1/documents/categories", body);
      }

      setModalOpen(false);
      fetchCategories();
    } catch (err: any) {
      setFormError(err.message || "Gagal menyimpan kategori.");
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.delete(`/api/v1/documents/categories/${deleteTarget.id}`);
      setDeleteTarget(null);
      fetchCategories();
    } catch (err: any) {
      alert("Gagal menghapus kategori: " + (err.message || "Kategori masih digunakan berkas."));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Manajemen Kategori Berkas</h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Konfigurasi 16 jenis berkas arsip dan batasan hak peran yang diizinkan mengunggah
          </p>
        </div>
        <button
          onClick={openCreateModal}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-xs shadow-sm shadow-indigo-200 transition-colors"
        >
          <FolderPlus className="w-4 h-4" />
          <span>Tambah Kategori</span>
        </button>
      </div>

      {/* Categories Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat kategori...</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Nama Kategori</th>
                  <th className="px-6 py-3.5">Tipe Kategori</th>
                  <th className="px-6 py-3.5">Peran Yang Diizinkan</th>
                  <th className="px-6 py-3.5 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {categories.map((c) => (
                  <tr key={c.id} className="hover:bg-slate-50/60 transition-colors">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center font-bold text-xs">
                          <Tag className="w-4 h-4" />
                        </div>
                        <div>
                          <p className="font-semibold text-slate-900">{c.name}</p>
                          <p className="text-xs text-slate-400 font-mono">ID: #{c.id}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      {c.is_operational ? (
                        <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-amber-100 text-amber-800">
                          Operasional BAAK
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-100 text-slate-700">
                          Akademik Reguler
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex flex-wrap gap-1 max-w-md">
                        {c.allowed_roles?.map((r) => (
                          <span
                            key={r}
                            className="px-2 py-0.5 rounded text-[11px] font-medium bg-slate-100 text-slate-700"
                          >
                            {r}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => openEditModal(c)}
                          className="p-1.5 text-slate-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
                          title="Ubah Kategori"
                        >
                          <Edit2 className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => setDeleteTarget(c)}
                          className="p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                          title="Hapus Kategori"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal Add/Edit */}
      {modalOpen && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg p-6 animate-in fade-in zoom-in-95 duration-150">
            <h3 className="text-lg font-bold text-slate-900">
              {editingId ? "Ubah Kategori Berkas" : "Tambah Kategori Berkas"}
            </h3>
            <p className="text-xs text-slate-500 mt-0.5">
              Tentukan nama kategori dan peran yang berwenang mengunggah berkas ini.
            </p>

            {formError && (
              <div className="mt-3 p-3 rounded-xl bg-red-50 border border-red-200 text-red-700 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="mt-4 space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Nama Kategori <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Contoh: Silabus dan RPS"
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-sm focus:ring-2 focus:ring-indigo-200"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-2">
                  Peran Yang Diizinkan Mengunggah <span className="text-red-500">*</span>
                </label>
                <div className="grid grid-cols-2 gap-2">
                  {ALL_ROLES.map((role) => (
                    <label
                      key={role.key}
                      className="flex items-center gap-2 p-2.5 rounded-lg border border-slate-200 text-xs font-medium cursor-pointer hover:bg-slate-50"
                    >
                      <input
                        type="checkbox"
                        checked={allowedRoles.includes(role.key)}
                        onChange={() => toggleRole(role.key)}
                        className="rounded text-indigo-600 focus:ring-indigo-500"
                      />
                      <span>{role.label}</span>
                    </label>
                  ))}
                </div>
              </div>

              <div>
                <label className="flex items-center gap-2 cursor-pointer p-3 rounded-lg border border-slate-200 text-xs font-semibold text-slate-800">
                  <input
                    type="checkbox"
                    checked={isOperational}
                    onChange={(e) => setIsOperational(e.target.checked)}
                    className="rounded text-indigo-600 focus:ring-indigo-500"
                  />
                  <span>Kategori Berkas Operasional BAAK</span>
                </label>
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
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
                  {formLoading ? "Menyimpan..." : "Simpan Kategori"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-red-50 text-red-600 flex items-center justify-center mb-4">
              <AlertCircle className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Hapus Kategori?</h3>
            <p className="text-xs text-slate-500 mt-1">
              Anda akan menghapus kategori <strong className="text-slate-800">{deleteTarget.name}</strong>.
              Penghapusan akan ditolak jika masih terdapat berkas yang menggunakannya.
            </p>

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                onClick={() => setDeleteTarget(null)}
                disabled={deleting}
                className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-xs font-semibold flex items-center gap-2"
              >
                {deleting ? "Menghapus..." : "Hapus Kategori"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
