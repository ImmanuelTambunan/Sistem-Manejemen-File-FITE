const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// IN-MEMORY TOKEN STORAGE (NO LOCALSTORAGE FOR XSS DEFENSE)
let inMemoryAccessToken: string | null = null;
let onUnauthorizedCallback: (() => void) | null = null;

export function setAccessToken(token: string | null) {
  inMemoryAccessToken = token;
}

export function getAccessToken(): string | null {
  return inMemoryAccessToken;
}

export function setOnUnauthorized(cb: () => void) {
  onUnauthorizedCallback = cb;
}

export function getApiUrl(path: string): string {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  // If path doesn't start with /api/v1 and doesn't start with /healthz, prefix with /api/v1
  if (!cleanPath.startsWith("/api/v1") && !cleanPath.startsWith("/healthz") && !cleanPath.startsWith("/readyz")) {
    return `${API_BASE_URL}/api/v1${cleanPath}`;
  }
  return `${API_BASE_URL}${cleanPath}`;
}

let isRefreshing = false;
let refreshSubscribers: ((token: string | null) => void)[] = [];

function subscribeTokenRefresh(cb: (token: string | null) => void) {
  refreshSubscribers.push(cb);
}

function onRefreshed(token: string | null) {
  refreshSubscribers.forEach((cb) => cb(token));
  refreshSubscribers = [];
}

export async function refreshAccessToken(): Promise<string | null> {
  try {
    const res = await fetch(getApiUrl("/auth/refresh"), {
      method: "POST",
      credentials: "include", // sends httpOnly refresh_token cookie
      headers: {
        Accept: "application/json",
      },
    });

    if (!res.ok) {
      setAccessToken(null);
      if (onUnauthorizedCallback) onUnauthorizedCallback();
      return null;
    }

    const json = await res.json();
    const token = json.data?.access_token || null;
    setAccessToken(token);
    return token;
  } catch {
    setAccessToken(null);
    if (onUnauthorizedCallback) onUnauthorizedCallback();
    return null;
  }
}

export interface ApiFetchOptions extends RequestInit {
  skipAuth?: boolean;
}

export async function apiFetch<T = any>(
  endpoint: string,
  options: ApiFetchOptions = {}
): Promise<{ success: boolean; message: string; data: T; meta?: any; code?: string; errors?: any }> {
  const url = getApiUrl(endpoint);
  const headers = new Headers(options.headers || {});

  if (!options.skipAuth && inMemoryAccessToken) {
    headers.set("Authorization", `Bearer ${inMemoryAccessToken}`);
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }

  // If body is not FormData and not already Content-Type set, default to application/json
  if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  let res = await fetch(url, {
    ...options,
    headers,
    credentials: "include",
  });

  // Handle 401 with Token Refresh & Single Retry
  if (res.status === 401 && !options.skipAuth && !endpoint.includes("/auth/login") && !endpoint.includes("/auth/refresh")) {
    if (!isRefreshing) {
      isRefreshing = true;
      const newToken = await refreshAccessToken();
      isRefreshing = false;
      onRefreshed(newToken);

      if (newToken) {
        headers.set("Authorization", `Bearer ${newToken}`);
        res = await fetch(url, { ...options, headers, credentials: "include" });
      }
    } else {
      // Wait for refresh to complete
      const retryToken = await new Promise<string | null>((resolve) => {
        subscribeTokenRefresh((token) => resolve(token));
      });
      if (retryToken) {
        headers.set("Authorization", `Bearer ${retryToken}`);
        res = await fetch(url, { ...options, headers, credentials: "include" });
      }
    }
  }

  const contentType = res.headers.get("Content-Type") || "";
  let body: any = null;
  if (contentType.includes("application/json")) {
    body = await res.json();
  } else {
    body = await res.text();
  }

  if (!res.ok) {
    const error: any = new Error(body?.message || "Terjadi kesalahan pada permintaan.");
    error.status = res.status;
    error.code = body?.code;
    error.errors = body?.errors;
    error.retryAfter = res.headers.get("Retry-After");
    error.data = body;
    throw error;
  }

  return body;
}

// Convenient API Client Wrapper
export const api = {
  get: <T = any>(endpoint: string, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, { method: "GET", ...options }),

  post: <T = any>(endpoint: string, body?: any, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, {
      method: "POST",
      body: body ? JSON.stringify(body) : undefined,
      ...options,
    }),

  put: <T = any>(endpoint: string, body?: any, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, {
      method: "PUT",
      body: body ? JSON.stringify(body) : undefined,
      ...options,
    }),

  patch: <T = any>(endpoint: string, body?: any, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, {
      method: "PATCH",
      body: body ? JSON.stringify(body) : undefined,
      ...options,
    }),

  delete: <T = any>(endpoint: string, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, { method: "DELETE", ...options }),

  postForm: <T = any>(endpoint: string, formData: FormData, options?: ApiFetchOptions) =>
    apiFetch<T>(endpoint, {
      method: "POST",
      body: formData,
      ...options,
    }),

  downloadBlob: async (endpoint: string): Promise<Blob> => {
    const url = getApiUrl(endpoint);
    const headers = new Headers();
    if (inMemoryAccessToken) {
      headers.set("Authorization", `Bearer ${inMemoryAccessToken}`);
    }
    const res = await fetch(url, {
      method: "GET",
      headers,
      credentials: "include",
    });
    if (!res.ok) {
      throw new Error(`Gagal mengunduh berkas (Status: ${res.status})`);
    }
    return res.blob();
  },
};
