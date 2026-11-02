export type Role = "super_admin" | "dekan" | "baak" | "kaprodi" | "dosen" | "mahasiswa";

export interface MenuItem {
  title: string;
  href: string;
  iconName: string;
  roles: Role[];
}

export const MENU_ITEMS: MenuItem[] = [
  {
    title: "Dashboard",
    href: "/dashboard",
    iconName: "LayoutDashboard",
    roles: ["super_admin", "dekan", "baak", "kaprodi", "dosen", "mahasiswa"],
  },
  {
    title: "File Explorer",
    href: "/berkas",
    iconName: "FolderArchive",
    roles: ["super_admin", "dekan", "baak", "kaprodi", "dosen", "mahasiswa"],
  },
  {
    title: "Explorer Publik",
    href: "/publik",
    iconName: "Globe",
    roles: ["super_admin", "dekan", "baak", "kaprodi", "dosen", "mahasiswa"],
  },
  {
    title: "Verifikasi Berkas",
    href: "/verifikasi",
    iconName: "CheckSquare",
    roles: ["super_admin", "baak"],
  },
  {
    title: "Notifikasi",
    href: "/notifikasi",
    iconName: "Bell",
    roles: ["super_admin", "dekan", "baak", "kaprodi", "dosen", "mahasiswa"],
  },
  {
    title: "Manajemen Pengguna",
    href: "/admin/pengguna",
    iconName: "Users",
    roles: ["super_admin"],
  },
  {
    title: "Manajemen Kategori",
    href: "/admin/kategori",
    iconName: "Tag",
    roles: ["super_admin"],
  },
  {
    title: "Program Studi",
    href: "/admin/prodi",
    iconName: "GraduationCap",
    roles: ["super_admin"],
  },
  {
    title: "Log Penyimpanan",
    href: "/admin/log",
    iconName: "HardDrive",
    roles: ["super_admin"],
  },
  {
    title: "Profil Saya",
    href: "/profil",
    iconName: "User",
    roles: ["super_admin", "dekan", "baak", "kaprodi", "dosen", "mahasiswa"],
  },
];

export function hasAccess(role: Role, href: string): boolean {
  const item = MENU_ITEMS.find((m) => href.startsWith(m.href));
  if (!item) return true;
  return item.roles.includes(role);
}
