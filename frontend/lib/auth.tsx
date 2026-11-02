"use client";

import React, { createContext, useContext, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { apiFetch, refreshAccessToken, setAccessToken, setOnUnauthorized } from "./api";
import { Role } from "./permissions";

export interface User {
  id: number;
  name: string;
  email: string;
  role: Role;
  role_label: string;
  nim?: string | null;
  nip?: string | null;
  phone?: string | null;
  prodi?: { id: number; code: string; name: string } | null;
  is_active: boolean;
  must_change_password: boolean;
}

interface AuthContextType {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<User>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
  setUser: React.Dispatch<React.SetStateAction<User | null>>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  const handleUnauthorized = () => {
    setUser(null);
    setAccessToken(null);
    if (!pathname.startsWith("/login")) {
      router.push("/login?expired=1");
    }
  };

  useEffect(() => {
    setOnUnauthorized(handleUnauthorized);

    async function initAuth() {
      try {
        const token = await refreshAccessToken();
        if (token) {
          const res = await apiFetch<User>("/auth/me");
          setUser(res.data);
        }
      } catch {
        setUser(null);
      } finally {
        setLoading(false);
      }
    }

    initAuth();
  }, []);

  const login = async (email: string, password: string): Promise<User> => {
    const res = await apiFetch<{
      access_token: string;
      token_type: string;
      expires_in: number;
      user: User;
    }>("/auth/login", {
      method: "POST",
      skipAuth: true,
      body: JSON.stringify({ email, password }),
    });

    setAccessToken(res.data.access_token);
    setUser(res.data.user);

    if (res.data.user.must_change_password) {
      router.push("/ganti-sandi");
    } else {
      router.push("/dashboard");
    }

    return res.data.user;
  };

  const logout = async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } catch {
      // ignore
    } finally {
      setAccessToken(null);
      setUser(null);
      router.push("/login");
    }
  };

  const refreshUser = async () => {
    try {
      const res = await apiFetch<User>("/auth/me");
      setUser(res.data);
    } catch {
      handleUnauthorized();
    }
  };

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, refreshUser, setUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
