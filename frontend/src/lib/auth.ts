const TOKEN_KEY = "authToken";

export function getToken() {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    return;
  }
}

async function readErrorMessage(res: Response) {
  try {
    const data = (await res.json()) as unknown;
    if (data && typeof data === "object" && "message" in data && typeof (data as any).message === "string") {
      return String((data as any).message);
    }
  } catch {
    return null;
  }
  return null;
}

export async function apiJson<T>(path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const headers = new Headers(init?.headers);
  headers.set("Accept", "application/json");
  if (init?.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(path, { ...init, headers });
  if (res.status === 401) clearToken();
  if (!res.ok) {
    const msg = (await readErrorMessage(res)) ?? `${res.status} ${res.statusText}`;
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as unknown as T;
  return (await res.json()) as T;
}

