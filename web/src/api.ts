import type {
  AnalysisListItem,
  AnalysisStatus,
  AuthResponse,
  BusinessContext,
  SignedURLResponse,
  StartAnalyzeRequest,
  User,
} from './types';

const API = import.meta.env.VITE_API_URL || '';

const TOKEN_KEY = 'va_token';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init?.headers as Record<string, string> || {}),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const r = await fetch(`${API}${path}`, { ...init, headers });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`${r.status} ${path}: ${text}`);
  }
  if (r.status === 204) return undefined as T;
  return r.json();
}

// Auth
export async function register(email: string, password: string): Promise<AuthResponse> {
  return req('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  return req('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export async function getMe(): Promise<User> {
  return req('/api/auth/me');
}

export async function updateMe(bc: BusinessContext): Promise<void> {
  return req('/api/auth/me', {
    method: 'PUT',
    body: JSON.stringify(bc),
  });
}

// Video analysis
export async function getSignedURL(filename: string, contentType: string): Promise<SignedURLResponse> {
  return req('/api/uploads/signed-url', {
    method: 'POST',
    body: JSON.stringify({ filename, content_type: contentType }),
  });
}

export function uploadToGCS(
  putURL: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', putURL);
    xhr.setRequestHeader('Content-Type', file.type || 'video/mp4');
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded / e.total);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`upload failed: ${xhr.status} ${xhr.responseText}`));
    };
    xhr.onerror = () => reject(new Error('upload network error'));
    xhr.send(file);
  });
}

export async function startAnalyze(req_: StartAnalyzeRequest): Promise<{ id: string; status: string }> {
  return req('/api/analyze', { method: 'POST', body: JSON.stringify(req_) });
}

export async function getAnalysis(id: string): Promise<AnalysisStatus> {
  return req(`/api/analyze/${id}`);
}

export async function listAnalyses(): Promise<AnalysisListItem[]> {
  return req('/api/analyses');
}
