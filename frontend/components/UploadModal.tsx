"use client";

import React, { useState, useEffect, useRef } from "react";
import {
  X,
  UploadCloud,
  AlertCircle,
  FileText,
  CheckCircle2,
  AlertTriangle,
  UserCheck,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";

interface Category {
  id?: number;
  ID?: number;
  name?: string;
  Name?: string;
  nama_kategori?: string;
  nama?: string;
  code?: string;
  Code?: string;
  kode?: string;
  allowed_roles?: string[];
  AllowedRoles?: string[];
  is_operational?: boolean;
  IsOperational?: boolean;
}

interface Prodi {
  id?: number;
  ID?: number;
  name?: string;
  Name?: string;
  nama_prodi?: string;
  nama?: string;
  code?: string;
  Code?: string;
  kode?: string;
}

interface UploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const ALLOWED_EXTS = [
  ".pdf",
  ".docx",
  ".xlsx",
  ".pptx",
  ".zip",
  ".png",
  ".jpg",
  ".jpeg",
];
const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

export default function UploadModal({
  isOpen,
  onClose,
  onSuccess,
}: UploadModalProps) {
  const { user } = useAuth();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [categories, setCategories] = useState<Category[]>([]);
  const [prodis, setProdis] = useState<Prodi[]>([]);
  const [loadingMeta, setLoadingMeta] = useState(false);

  const [file, setFile] = useState<File | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const [title, setTitle] = useState("");
  const [categoryId, setCategoryId] = useState<number | "">("");
  const [prodiId, setProdiId] = useState<number | "">("");
  const [visibility, setVisibility] = useState<"private" | "public">("private");
  const [description, setDescription] = useState("");
  const [archivedAt, setArchivedAt] = useState("");
  const [subjectUserId, setSubjectUserId] = useState("");

  const isPrivileged = user?.role === "super_admin" || user?.role === "baak";

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (isOpen) {
      setError(null);
      setFieldErrors({});
      loadMetadata();
      if (user?.prodi?.id) {
        setProdiId(user.prodi.id);
      }
    } else {
      // reset form
      setFile(null);
      setTitle("");
      setCategoryId("");
      setVisibility("private");
      setDescription("");
      setArchivedAt("");
      setSubjectUserId("");
      setProdiId(user?.prodi?.id || "");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, user]);

  const loadMetadata = async () => {
    setLoadingMeta(true);
    try {
      const [catRes, prodiRes] = await Promise.all([
        api.get<Category[]>("/api/v1/documents/categories"),
        api.get<Prodi[]>("/api/v1/users/prodi"),
      ]);

      const allCats = catRes.data || [];
      // Filter category by allowed_roles
      const userRole = user?.role || "";
      const filteredCats = allCats.filter((cat: any) => {
        const roles = cat.allowed_roles || cat.AllowedRoles;
        if (!roles || roles.length === 0) return true;
        return roles.includes(userRole);
      });

      setCategories(filteredCats);
      setProdis(prodiRes.data || []);
      if (filteredCats.length > 0 && !categoryId) {
        const firstId = filteredCats[0].id ?? (filteredCats[0] as any).ID;
        if (firstId) {
          setCategoryId(firstId);
        }
      }
    } catch (err: any) {
      console.error("Gagal memuat kategori:", err);
    } finally {
      setLoadingMeta(false);
    }
  };

  if (!isOpen) return null;

  const validateFile = (selectedFile: File): string | null => {
    const ext = "." + selectedFile.name.split(".").pop()?.toLowerCase();
    if (!ALLOWED_EXTS.includes(ext)) {
      return `Format berkas "${ext}" tidak diizinkan. Format yang didukung: ${ALLOWED_EXTS.join(", ")}`;
    }
    if (selectedFile.size > MAX_FILE_SIZE) {
      return `Ukuran berkas (${(selectedFile.size / (1024 * 1024)).toFixed(2)} MB) melebihi batas maksimal 10 MB.`;
    }
    return null;
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const selected = e.target.files[0];
      const err = validateFile(selected);
      if (err) {
        setError(err);
        setFile(null);
      } else {
        setError(null);
        setFile(selected);
        if (!title) {
          // prefill title from file name without extension
          const nameWithoutExt = selected.name.replace(/\.[^/.]+$/, "");
          setTitle(nameWithoutExt);
        }
      }
    }
  };

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const selected = e.dataTransfer.files[0];
      const err = validateFile(selected);
      if (err) {
        setError(err);
        setFile(null);
      } else {
        setError(null);
        setFile(selected);
        if (!title) {
          setTitle(selected.name.replace(/\.[^/.]+$/, ""));
        }
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setFieldErrors({});

    if (!file) {
      setError("Silakan pilih berkas yang akan diunggah.");
      return;
    }
    if (!title.trim()) {
      setFieldErrors((prev) => ({
        ...prev,
        title: "Judul berkas wajib diisi.",
      }));
      return;
    }
    if (!categoryId) {
      setFieldErrors((prev) => ({
        ...prev,
        category_id: "Kategori berkas wajib dipilih.",
      }));
      return;
    }
    if (isPrivileged && !subjectUserId && !prodiId) {
      setFieldErrors((prev) => ({
        ...prev,
        prodi_id: "Program studi wajib dipilih untuk arsip fakultas/admin.",
      }));
      setError("Silakan pilih Program Studi terlebih dahulu.");
      return;
    }

    setSubmitting(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("title", title.trim());
      formData.append("category_id", String(categoryId));
      formData.append("visibility", visibility);
      if (description.trim()) {
        formData.append("description", description.trim());
      }
      if (prodiId) {
        formData.append("prodi_id", String(prodiId));
      }
      if (isPrivileged) {
        if (subjectUserId.trim()) {
          formData.append("subject_user_id", subjectUserId.trim());
        }
        if (archivedAt) {
          formData.append("archived_at", archivedAt);
        }
      }

      await api.postForm("/api/v1/documents", formData);

      onSuccess();
      onClose();
    } catch (err: any) {
      console.error("Gagal unggah dokumen:", err);
      if (err.data?.errors) {
        setFieldErrors(err.data.errors);
      }
      const backendMsg = err.data?.message || err.message;
      setError(
        backendMsg ||
          "Gagal mengunggah berkas. Periksa kembali isian formulir.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-slate-900/60 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100 bg-slate-50/50">
          <div>
            <h3 className="text-lg font-bold text-slate-800">
              Unggah Berkas Digital
            </h3>
            <p className="text-xs text-slate-500 mt-0.5">
              Simpan dokumen ke sistem arsip digital Fakultas FITE
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {error && (
            <div className="p-3.5 rounded-xl bg-red-50 border border-red-200 flex items-start gap-3 text-red-700 text-sm">
              <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          {/* Drag & Drop File Zone */}
          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-2">
              Pilih Berkas <span className="text-red-500">*</span>
            </label>
            <div
              onDragEnter={handleDrag}
              onDragOver={handleDrag}
              onDragLeave={handleDrag}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              className={`border-2 border-dashed rounded-xl p-6 text-center cursor-pointer transition-all ${
                dragActive
                  ? "border-indigo-500 bg-indigo-50/50"
                  : file
                    ? "border-emerald-400 bg-emerald-50/30"
                    : "border-slate-300 hover:border-indigo-400 bg-slate-50/50 hover:bg-slate-50"
              }`}
            >
              <input
                ref={fileInputRef}
                type="file"
                className="hidden"
                accept={ALLOWED_EXTS.join(",")}
                onChange={handleFileChange}
              />
              {file ? (
                <div className="flex items-center justify-center gap-3">
                  <div className="w-10 h-10 rounded-lg bg-emerald-100 text-emerald-600 flex items-center justify-center">
                    <FileText className="w-5 h-5" />
                  </div>
                  <div className="text-left">
                    <p className="text-sm font-semibold text-slate-800 line-clamp-1">
                      {file.name}
                    </p>
                    <p className="text-xs text-slate-500">
                      {(file.size / (1024 * 1024)).toFixed(2)} MB • Klik untuk
                      ganti
                    </p>
                  </div>
                  <CheckCircle2 className="w-5 h-5 text-emerald-500 ml-auto" />
                </div>
              ) : (
                <div className="flex flex-col items-center">
                  <div className="w-12 h-12 rounded-full bg-indigo-50 text-indigo-600 flex items-center justify-center mb-3">
                    <UploadCloud className="w-6 h-6" />
                  </div>
                  <p className="text-sm font-medium text-slate-700">
                    Tarik dan lepaskan berkas di sini, atau{" "}
                    <span className="text-indigo-600 font-semibold underline">
                      pilih dari perangkat
                    </span>
                  </p>
                  <p className="text-xs text-slate-400 mt-1">
                    PDF, DOCX, XLSX, PPTX, ZIP, PNG, JPG (Maks. 10 MB)
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* Title & Category Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
                Judul Berkas <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="Contoh: Surat Keputusan Pembimbing Skripsi"
                className={`w-full px-3.5 py-2.5 rounded-lg border text-sm text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 ${
                  fieldErrors.title
                    ? "border-red-300 focus:ring-red-200"
                    : "border-slate-300 focus:ring-indigo-200 focus:border-indigo-500"
                }`}
                required
              />
              {fieldErrors.title && (
                <p className="text-xs text-red-500 mt-1">{fieldErrors.title}</p>
              )}
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
                Kategori Berkas <span className="text-red-500">*</span>
              </label>
              <select
                value={categoryId}
                onChange={(e) => setCategoryId(Number(e.target.value))}
                disabled={loadingMeta}
                className="w-full px-3.5 py-2.5 rounded-lg border border-slate-300 text-sm text-slate-800 bg-white focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
                required
              >
                <option value="" disabled>
                  Pilih Kategori
                </option>
                {categories.map((cat: any) => {
                  const catId = cat.id ?? cat.ID;
                  const catName =
                    cat.name ||
                    cat.Name ||
                    cat.nama_kategori ||
                    cat.nama ||
                    `Kategori ${catId}`;
                  const catCode = cat.code || cat.Code || cat.kode;
                  return (
                    <option key={catId} value={catId}>
                      {catName}{catCode ? ` (${catCode})` : ""}
                    </option>
                  );
                })}
              </select>
            </div>
          </div>

          {/* Program Studi Selector for Privileged / Users without fixed prodi */}
          {(!user?.prodi?.id || isPrivileged) && (
            <div>
              <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
                Program Studi{" "}
                {isPrivileged && !subjectUserId && (
                  <span className="text-red-500">*</span>
                )}
              </label>
              <select
                value={prodiId}
                onChange={(e) =>
                  setProdiId(e.target.value ? Number(e.target.value) : "")
                }
                disabled={loadingMeta}
                className={`w-full px-3.5 py-2.5 rounded-lg border text-sm text-slate-800 bg-white focus:outline-none focus:ring-2 ${
                  fieldErrors.prodi_id
                    ? "border-red-300 focus:ring-red-200"
                    : "border-slate-300 focus:ring-indigo-200 focus:border-indigo-500"
                }`}
              >
                <option value="">
                  {subjectUserId
                    ? "-- Otomatis mengikuti prodi pemilik subjek (opsional) --"
                    : "-- Pilih Program Studi --"}
                </option>
                {prodis.map((p: any) => {
                  const pId = p.id ?? p.ID;
                  const pName =
                    p.name || p.Name || p.nama_prodi || p.nama || `Prodi ${pId}`;
                  const pCode = (p.code || p.Code || p.kode || "").toUpperCase();
                  return (
                    <option key={pId} value={pId}>
                      {pName}{pCode ? ` (${pCode})` : ""}
                    </option>
                  );
                })}
              </select>
              {fieldErrors.prodi_id && (
                <p className="text-xs text-red-500 mt-1">
                  {fieldErrors.prodi_id}
                </p>
              )}
            </div>
          )}

          {/* Privileged Archival Configuration (BAAK & Super Admin - UC-23) */}
          {isPrivileged && (
            <div className="p-4 rounded-xl bg-indigo-50/40 border border-indigo-100 space-y-3">
              <div className="flex items-center gap-2">
                <UserCheck className="w-4 h-4 text-indigo-600" />
                <span className="text-xs font-bold text-indigo-950 uppercase tracking-wider">
                  Pengaturan Khusus BAAK & Admin (Arsip Atas Nama & Retroaktif)
                </span>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {/* Subject User ID */}
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    ID Pengguna Sivitas (Subjek Pemilik)
                  </label>
                  <input
                    type="number"
                    min="1"
                    value={subjectUserId}
                    onChange={(e) => setSubjectUserId(e.target.value)}
                    placeholder="Contoh: 8 (Kosongkan jika atas nama sendiri)"
                    className={`w-full px-3.5 py-2 rounded-lg border text-sm text-slate-800 bg-white placeholder-slate-400 focus:outline-none focus:ring-2 ${
                      fieldErrors.subject_user_id
                        ? "border-red-300 focus:ring-red-200"
                        : "border-slate-300 focus:ring-indigo-200 focus:border-indigo-500"
                    }`}
                  />
                  {fieldErrors.subject_user_id ? (
                    <p className="text-xs text-red-500 mt-1">
                      {fieldErrors.subject_user_id}
                    </p>
                  ) : (
                    <p className="text-[11px] text-slate-500 mt-1">
                      ID mahasiswa/dosen sasaran untuk pengarsipan oleh BAAK.
                    </p>
                  )}
                </div>

                {/* Tanggal Arsip Retroaktif */}
                <div>
                  <label className="block text-xs font-semibold text-slate-700 mb-1">
                    Tanggal Arsip Retroaktif (Lampau)
                  </label>
                  <input
                    type="date"
                    value={archivedAt}
                    max={new Date().toISOString().split("T")[0]}
                    onChange={(e) => setArchivedAt(e.target.value)}
                    className={`w-full px-3.5 py-2 rounded-lg border text-sm text-slate-800 bg-white focus:outline-none focus:ring-2 ${
                      fieldErrors.archived_at
                        ? "border-red-300 focus:ring-red-200"
                        : "border-slate-300 focus:ring-indigo-200 focus:border-indigo-500"
                    }`}
                  />
                  {fieldErrors.archived_at ? (
                    <p className="text-xs text-red-500 mt-1">
                      {fieldErrors.archived_at}
                    </p>
                  ) : (
                    <p className="text-[11px] text-slate-500 mt-1">
                      Kosongkan jika diarsipkan per tanggal hari ini.
                    </p>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Visibility Option */}
          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-2">
              Akses & Visibilitas
            </label>
            <div className="grid grid-cols-2 gap-3">
              <label
                className={`flex items-start gap-3 p-3.5 rounded-xl border cursor-pointer transition-all ${
                  visibility === "private"
                    ? "border-indigo-600 bg-indigo-50/40 text-slate-900 ring-1 ring-indigo-600"
                    : "border-slate-200 hover:border-slate-300 text-slate-600"
                }`}
              >
                <input
                  type="radio"
                  name="visibility"
                  value="private"
                  checked={visibility === "private"}
                  onChange={() => setVisibility("private")}
                  className="mt-0.5 text-indigo-600 focus:ring-indigo-500"
                />
                <div>
                  <span className="block text-sm font-semibold text-slate-800">
                    Internal / Terbatas
                  </span>
                  <span className="block text-xs text-slate-500 mt-0.5">
                    Hanya dapat diakses oleh pihak berwenang sesuai hak akses
                    peran.
                  </span>
                </div>
              </label>

              <label
                className={`flex items-start gap-3 p-3.5 rounded-xl border cursor-pointer transition-all ${
                  visibility === "public"
                    ? "border-indigo-600 bg-indigo-50/40 text-slate-900 ring-1 ring-indigo-600"
                    : "border-slate-200 hover:border-slate-300 text-slate-600"
                }`}
              >
                <input
                  type="radio"
                  name="visibility"
                  value="public"
                  checked={visibility === "public"}
                  onChange={() => setVisibility("public")}
                  className="mt-0.5 text-indigo-600 focus:ring-indigo-500"
                />
                <div>
                  <span className="block text-sm font-semibold text-slate-800">
                    Publik (Explorer)
                  </span>
                  <span className="block text-xs text-slate-500 mt-0.5">
                    Dapat dilihat publik setelah diverifikasi BAAK/Admin.
                  </span>
                </div>
              </label>
            </div>

            {visibility === "public" && (
              <div className="mt-3 p-3 rounded-xl bg-amber-50 border border-amber-200 flex items-start gap-2.5 text-amber-800 text-xs">
                <AlertTriangle className="w-4 h-4 text-amber-600 flex-shrink-0 mt-0.5" />
                <span>
                  <strong>Perhatian:</strong> Berkas bertanda <em>Publik</em>{" "}
                  akan berstatus{" "}
                  <code className="bg-amber-100 text-amber-900 px-1 py-0.5 rounded font-mono">
                    menunggu verifikasi
                  </code>
                  . Berkas tidak akan langsung muncul di katalog publik sebelum
                  disetujui oleh BAAK atau Super Admin.
                </span>
              </div>
            )}
          </div>

          {/* Description & Optional Fields */}
          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Deskripsi Berkas (Opsional)
            </label>
            <textarea
              rows={2}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Catatan tambahan mengenai berkas ini..."
              className="w-full px-3.5 py-2 rounded-lg border border-slate-300 text-sm text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-indigo-200 focus:border-indigo-500"
            />
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
            <button
              type="button"
              onClick={onClose}
              disabled={submitting}
              className="px-4 py-2.5 rounded-lg border border-slate-300 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={submitting || !file}
              className="px-5 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-semibold shadow-sm shadow-indigo-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 transition-colors"
            >
              {submitting ? (
                <>
                  <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  <span>Mengunggah...</span>
                </>
              ) : (
                <span>Unggah Berkas</span>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
