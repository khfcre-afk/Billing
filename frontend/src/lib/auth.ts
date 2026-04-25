import { create } from "zustand";
import { persist } from "zustand/middleware";
import { api } from "./api";

export type Me = {
  id: string;
  email: string;
  role: "user" | "admin";
  balance_cents: number;
  created_at: string;
};

type AuthState = {
  me: Me | null;
  loading: boolean;
  bootstrap: () => Promise<void>;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
};

export const useAuth = create<AuthState>()(
  persist(
    (set, get) => ({
      me: null,
      loading: true,
      bootstrap: async () => {
        try {
          const me = await api.get<Me>("/users/me");
          set({ me, loading: false });
        } catch {
          set({ me: null, loading: false });
        }
      },
      login: async (email, password) => {
        await api.post("/auth/login", { email, password });
        await get().refresh();
      },
      register: async (email, password) => {
        await api.post("/auth/register", { email, password });
        await get().refresh();
      },
      logout: async () => {
        try {
          await api.post("/auth/logout");
        } catch {
          /* ignore */
        }
        set({ me: null });
      },
      refresh: async () => {
        const me = await api.get<Me>("/users/me");
        set({ me });
      },
    }),
    { name: "auth-cache", partialize: (s) => ({ me: s.me }) },
  ),
);
