"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Users,
  Search,
  Plus,
  KeyRound,
  Shield,
  Edit2,
  Trash2,
  CheckCircle,
  XCircle,
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  Filter,
  UserCheck,
  UserX,
} from "lucide-react";
import { api } from "@/lib/api";

interface UserItem {
  id: number;
  name: string;
  email: string;
  role: string;
  role_label: string;
  nim?: string;
  nip?: string;
  phone?: string;
  prodi?: { id: number; name: string; code: string };
  is_active: boolean;
  must_change_password: boolean;
}

interface RoleItem {
  id: number;
  key: string;
  name: string;
}

interface ProdiItem {
  id: number;
  name: string;
  code: string;
}

interface Meta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export default function UserManagementPage() {
  const [users, setUsers] = useState<UserItem[]>([]);
  const [roles, setRoles] = useState<RoleItem[]>([]);
  const [prodis, setProdis] = useState<ProdiItem[]>([]);
  const [meta, setMeta] = useState<Meta>({ page: 1, per_page: 15, total: 0, total_pages: 1 });
  const [loading, setLoading] = useState(true);

  // Filters
  const [q, setQ] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const [prodiFilter, setProdiFilter] = useState("");
  const [activeFilter, setActiveFilter] = useState("");
  const [page, setPage] = useState(1);

  // Modals
  const [createOpen, setCreateOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<UserItem | null>(null);
  const [resetTarget, setResetTarget] = useState<UserItem | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<UserItem | null>(null);

  // Form State for Create/Edit
  const [formName, setFormName] = useState("");
  const [formEmail, setFormEmail] = useState("");
  const [formPassword, setFormPassword] = useState("");
  const [formRole, setFormRole] = useState("mahasiswa");
  const [formProdiId, setFormProdiId] = useState<number | "">("");
  const [formNim, setFormNim] = useState("");
  const [formNip, setFormNip] = useState("");
  const [formPhone, setFormPhone] = useState("");

  // Reset Password State
  const [resetPassword, setResetPassword] = useState("");
  const [resetConfirm, setResetConfirm] = useState("");

  const [formLoading, setFormLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (q.trim()) params.append("q", q.trim());
      if (roleFilter) params.append("role", roleFilter);
      if (prodiFilter) params.append("prodi", prodiFilter);
      if (activeFilter !== "") params.append("is_active", activeFilter);
      params.append("page", String(page));
      params.append("per_page", "15");

      const res = await api.get<UserItem[]>(
        `/api/v1/users?${params.toString()}`
      );
      setUsers(res.data || []);
      if (res.meta) setMeta(res.meta);
    } catch (err) {
      console.error("Gagal memuat pengguna:", err);
    } finally {
      setLoading(false);
    }
  }, [q, roleFilter, prodiFilter, activeFilter, page]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  useEffect(() => {
    api.get<RoleItem[]>("/api/v1/users/roles")
      .then((res) => setRoles(res.data || []))
      .catch(() => {});
    api.get<ProdiItem[]>("/api/v1/users/prodi")
      .then((res) => setProdis(res.data || []))
      .catch(() => {});
  }, []);

  const openCreateModal = () => {
    setFormName("");
    setFormEmail("");
    setFormPassword("ArsipFITE2026!");
    setFormRole("mahasiswa");
    setFormProdiId("");
    setFormNim("");
    setFormNip("");
    setFormPhone("");
    setFormError(null);
    setCreateOpen(true);
  };

  const openEditModal = (u: UserItem) => {
    setEditTarget(u);
    setFormName(u.name);
    setFormEmail(u.email);
    setFormRole(u.role);
    setFormProdiId(u.prodi?.id || "");
    setFormNim(u.nim || "");
    setFormNip(u.nip || "");
    setFormPhone(u.phone || "");
    setFormError(null);
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormLoading(true);
    setFormError(null);
    try {
      const body: any = {
        name: formName.trim(),
        email: formEmail.trim(),
        password: formPassword,
        role: formRole,
      };
      if (formProdiId) body.prodi_id = Number(formProdiId);
      if (formNim.trim()) body.nim = formNim.trim();
      if (formNip.trim()) body.nip = formNip.trim();
      if (formPhone.trim()) body.phone = formPhone.trim();

      await api.post("/api/v1/users", body);
      setCreateOpen(false);
      fetchUsers();
    } catch (err: any) {
      setFormError(err.message || "Gagal membuat pengguna.");
    } finally {
      setFormLoading(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editTarget) return;
    setFormLoading(true);
    setFormError(null);
    try {
      const body: any = {
        name: formName.trim(),
        email: formEmail.trim(),
      };
      if (formProdiId) body.prodi_id = Number(formProdiId);
      if (formNim.trim()) body.nim = formNim.trim();
      if (formNip.trim()) body.nip = formNip.trim();
      if (formPhone.trim()) body.phone = formPhone.trim();

      await api.put(`/api/v1/users/${editTarget.id}`, body);

      // If role changed
      if (formRole !== editTarget.role) {
        await api.patch(`/api/v1/users/${editTarget.id}/role`, {
          role: formRole,
          prodi_id: formProdiId ? Number(formProdiId) : null,
        });
      }

      setEditTarget(null);
      fetchUsers();
    } catch (err: any) {
      setFormError(err.message || "Gagal memperbarui pengguna.");
    } finally {
      setFormLoading(false);
    }
  };

  const handleToggleStatus = async (u: UserItem) => {
    try {
      await api.patch(`/api/v1/users/${u.id}/status`, { is_active: !u.is_active });
      setUsers((prev) =>
        prev.map((item) => (item.id === u.id ? { ...item, is_active: !u.is_active } : item))
      );
    } catch (err: any) {
      alert("Gagal memperbarui status: " + (err.message || "Akses ditolak."));
    }
  };

  const handleResetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resetTarget) return;
    if (resetPassword !== resetConfirm) {
      setFormError("Konfirmasi kata sandi tidak cocok.");
      return;
    }
    setFormLoading(true);
    setFormError(null);
    try {
      await api.patch(`/api/v1/users/${resetTarget.id}/password`, {
        password: resetPassword,
        password_confirmation: resetConfirm,
      });
      setResetTarget(null);
      setResetPassword("");
      setResetConfirm("");
      alert("Kata sandi berhasil direset. Pengguna wajib mengganti sandi pada login berikutnya.");
    } catch (err: any) {
      setFormError(err.message || "Gagal mereset kata sandi.");
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setFormLoading(true);
    try {
      await api.delete(`/api/v1/users/${deleteTarget.id}`);
      setDeleteTarget(null);
      fetchUsers();
    } catch (err: any) {
      alert("Gagal menghapus pengguna: " + (err.message || "Akses ditolak."));
    } finally {
      setFormLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold text-slate-900 tracking-tight">Manajemen Pengguna</h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Kelola akun, peran otorisasi, status aktif, dan pengaturan sandi sivitas akademika
          </p>
        </div>
        <button
          onClick={openCreateModal}
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-700 text-white font-semibold text-xs shadow-sm shadow-indigo-200 transition-colors"
        >
          <Plus className="w-4 h-4" />
          <span>Tambah Pengguna</span>
        </button>
      </div>

      {/* Toolbar Search & Filters */}
      <div className="bg-white rounded-2xl border border-slate-200/80 p-4 shadow-sm space-y-3">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
          <div className="md:col-span-2 relative">
            <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              value={q}
              onChange={(e) => {
                setQ(e.target.value);
                setPage(1);
              }}
              placeholder="Cari nama, email, NIM, atau NIP..."
              className="w-full pl-10 pr-4 py-2 rounded-xl border border-slate-200 text-xs focus:outline-none focus:ring-2 focus:ring-indigo-200"
            />
          </div>

          <div>
            <select
              value={roleFilter}
              onChange={(e) => {
                setRoleFilter(e.target.value);
                setPage(1);
              }}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
            >
              <option value="">Semua Peran</option>
              <option value="super_admin">Super Admin</option>
              <option value="dekan">Dekan</option>
              <option value="baak">BAAK</option>
              <option value="kaprodi">Kaprodi</option>
              <option value="dosen">Dosen</option>
              <option value="mahasiswa">Mahasiswa</option>
            </select>
          </div>

          <div>
            <select
              value={prodiFilter}
              onChange={(e) => {
                setProdiFilter(e.target.value);
                setPage(1);
              }}
              className="w-full px-3 py-2 rounded-xl border border-slate-200 text-xs bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200"
            >
              <option value="">Semua Prodi</option>
              <option value="IF">Informatika (IF)</option>
              <option value="SI">Sistem Informasi (SI)</option>
              <option value="TE">Teknik Elektro (TE)</option>
            </select>
          </div>
        </div>
      </div>

      {/* Users Table */}
      <div className="bg-white rounded-2xl border border-slate-200/80 shadow-sm overflow-hidden">
        {loading ? (
          <div className="p-16 text-center">
            <div className="w-10 h-10 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin mx-auto mb-3" />
            <p className="text-sm font-medium text-slate-500">Memuat data pengguna...</p>
          </div>
        ) : users.length === 0 ? (
          <div className="p-16 text-center">
            <Users className="w-14 h-14 text-slate-300 mx-auto mb-3" />
            <h4 className="text-base font-semibold text-slate-700">Tidak ada pengguna ditemukan</h4>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50/70 border-b border-slate-100 text-xs font-semibold text-slate-500 uppercase tracking-wider">
                <tr>
                  <th className="px-6 py-3.5">Nama & Identitas</th>
                  <th className="px-6 py-3.5">Email</th>
                  <th className="px-6 py-3.5">Peran</th>
                  <th className="px-6 py-3.5">Program Studi</th>
                  <th className="px-6 py-3.5">Status</th>
                  <th className="px-6 py-3.5 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 text-slate-700">
                {users.map((u) => (
                  <tr key={u.id} className="hover:bg-slate-50/60 transition-colors">
                    <td className="px-6 py-4">
                      <div>
                        <p className="font-semibold text-slate-900">{u.name}</p>
                        <p className="text-xs text-slate-400 font-mono">
                          {u.nim ? `NIM: ${u.nim}` : u.nip ? `NIP: ${u.nip}` : "ID: #" + u.id}
                        </p>
                      </div>
                    </td>
                    <td className="px-6 py-4 text-xs font-mono text-slate-600">{u.email}</td>
                    <td className="px-6 py-4">
                      <span className="inline-block px-2.5 py-0.5 rounded-full text-xs font-semibold bg-indigo-50 text-indigo-700">
                        {u.role_label}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-xs font-medium text-slate-600">
                      {u.prodi ? `${u.prodi.name} (${u.prodi.code})` : "-"}
                    </td>
                    <td className="px-6 py-4">
                      <button
                        onClick={() => handleToggleStatus(u)}
                        className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold transition-colors ${
                          u.is_active
                            ? "bg-emerald-100 text-emerald-800 hover:bg-emerald-200"
                            : "bg-red-100 text-red-800 hover:bg-red-200"
                        }`}
                        title="Klik untuk ubah status aktif"
                      >
                        {u.is_active ? <UserCheck className="w-3 h-3" /> : <UserX className="w-3 h-3" />}
                        <span>{u.is_active ? "Aktif" : "Nonaktif"}</span>
                      </button>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-1.5">
                        <button
                          onClick={() => {
                            setResetTarget(u);
                            setResetPassword("ArsipFITE2026!");
                            setResetConfirm("ArsipFITE2026!");
                            setFormError(null);
                          }}
                          className="p-1.5 text-slate-500 hover:text-amber-600 hover:bg-amber-50 rounded-lg transition-colors"
                          title="Reset Kata Sandi"
                        >
                          <KeyRound className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => openEditModal(u)}
                          className="p-1.5 text-slate-500 hover:text-indigo-600 hover:bg-indigo-50 rounded-lg transition-colors"
                          title="Ubah Data Pengguna"
                        >
                          <Edit2 className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => setDeleteTarget(u)}
                          className="p-1.5 text-slate-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                          title="Hapus Pengguna"
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

        {/* Pagination */}
        {meta.total_pages > 1 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-slate-100 bg-slate-50/50 text-xs">
            <span className="text-slate-500">
              Halaman {meta.page} dari {meta.total_pages} ({meta.total} pengguna)
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

      {/* Create / Edit User Modal */}
      {(createOpen || editTarget) && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg p-6 animate-in fade-in zoom-in-95 duration-150">
            <h3 className="text-lg font-bold text-slate-900">
              {createOpen ? "Tambah Pengguna Baru" : "Ubah Data Pengguna"}
            </h3>
            <p className="text-xs text-slate-500 mt-0.5">
              Isi data akun pengguna sesuai ketentuan peran di FITE.
            </p>

            {formError && (
              <div className="mt-3 p-3 rounded-xl bg-red-50 border border-red-200 text-red-700 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={createOpen ? handleCreate : handleUpdate} className="mt-4 space-y-3.5">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Nama Lengkap <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs focus:ring-2 focus:ring-indigo-200"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Email Akun <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs focus:ring-2 focus:ring-indigo-200"
                  required
                />
              </div>

              {createOpen && (
                <div>
                  <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                    Kata Sandi Awal <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={formPassword}
                    onChange={(e) => setFormPassword(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs font-mono focus:ring-2 focus:ring-indigo-200"
                    required
                  />
                  <p className="text-[11px] text-slate-400 mt-1">
                    Minimal 8 karakter (huruf besar, kecil, angka, simbol). Pengguna wajib ganti sandi saat login pertama.
                  </p>
                </div>
              )}

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                    Peran (Role) <span className="text-red-500">*</span>
                  </label>
                  <select
                    value={formRole}
                    onChange={(e) => setFormRole(e.target.value)}
                    className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs bg-white focus:ring-2 focus:ring-indigo-200"
                    required
                  >
                    <option value="mahasiswa">Mahasiswa</option>
                    <option value="dosen">Dosen</option>
                    <option value="kaprodi">Kaprodi</option>
                    <option value="baak">BAAK</option>
                    <option value="dekan">Dekan</option>
                    <option value="super_admin">Super Admin</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                    Program Studi
                  </label>
                  <select
                    value={formProdiId}
                    onChange={(e) => setFormProdiId(e.target.value ? Number(e.target.value) : "")}
                    className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs bg-white focus:ring-2 focus:ring-indigo-200"
                  >
                    <option value="">Tidak Terkait / Fakultas</option>
                    {prodis.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} ({p.code})
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                    NIM (Mahasiswa)
                  </label>
                  <input
                    type="text"
                    value={formNim}
                    onChange={(e) => setFormNim(e.target.value)}
                    placeholder="11S..."
                    className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs focus:ring-2 focus:ring-indigo-200"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                    NIP (Dosen/Tendik)
                  </label>
                  <input
                    type="text"
                    value={formNip}
                    onChange={(e) => setFormNip(e.target.value)}
                    placeholder="198..."
                    className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs focus:ring-2 focus:ring-indigo-200"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Nomor Telepon
                </label>
                <input
                  type="text"
                  value={formPhone}
                  onChange={(e) => setFormPhone(e.target.value)}
                  placeholder="08123456789 atau +628123456789"
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs focus:ring-2 focus:ring-indigo-200"
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => {
                    setCreateOpen(false);
                    setEditTarget(null);
                  }}
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
                  {formLoading ? "Menyimpan..." : "Simpan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reset Password Modal */}
      {resetTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md p-6 animate-in fade-in zoom-in-95 duration-150">
            <div className="w-12 h-12 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center mb-4">
              <KeyRound className="w-6 h-6" />
            </div>
            <h3 className="text-lg font-bold text-slate-900">Reset Kata Sandi Akun</h3>
            <p className="text-xs text-slate-500 mt-1">
              Reset sandi untuk <strong className="text-slate-800">{resetTarget.name}</strong> ({resetTarget.email}).
            </p>

            {formError && (
              <div className="mt-3 p-3 rounded-lg bg-red-50 text-red-700 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                <span>{formError}</span>
              </div>
            )}

            <form onSubmit={handleResetPassword} className="mt-4 space-y-3">
              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Kata Sandi Baru
                </label>
                <input
                  type="text"
                  value={resetPassword}
                  onChange={(e) => setResetPassword(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs font-mono focus:ring-2 focus:ring-amber-200"
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-700 uppercase mb-1">
                  Konfirmasi Sandi Baru
                </label>
                <input
                  type="text"
                  value={resetConfirm}
                  onChange={(e) => setResetConfirm(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg border border-slate-300 text-xs font-mono focus:ring-2 focus:ring-amber-200"
                  required
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setResetTarget(null)}
                  disabled={formLoading}
                  className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={formLoading}
                  className="px-4 py-2 rounded-lg bg-amber-600 hover:bg-amber-700 text-white text-xs font-semibold flex items-center gap-2"
                >
                  {formLoading ? "Mereset..." : "Reset Sandi"}
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
            <h3 className="text-lg font-bold text-slate-900">Hapus Akun Pengguna?</h3>
            <p className="text-xs text-slate-500 mt-1">
              Akun <strong className="text-slate-800">{deleteTarget.name}</strong> ({deleteTarget.email}) akan dinonaktifkan / dihapus dari sistem.
            </p>

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                onClick={() => setDeleteTarget(null)}
                disabled={formLoading}
                className="px-4 py-2 rounded-lg border border-slate-300 text-xs font-medium text-slate-700 hover:bg-slate-50"
              >
                Batal
              </button>
              <button
                onClick={handleDelete}
                disabled={formLoading}
                className="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white text-xs font-semibold flex items-center gap-2"
              >
                {formLoading ? "Menghapus..." : "Ya, Hapus Akun"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
