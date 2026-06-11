import { create } from "zustand"
import type { UserProfile } from "@/gen/darkside/v1/api_pb"

interface AuthState {
  user: UserProfile | null
  loading: boolean
  setUser: (u: UserProfile | null) => void
  setLoading: (v: boolean) => void
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  loading: true,
  setUser: (user) => set({ user }),
  setLoading: (loading) => set({ loading }),
}))

export async function fetchMe(): Promise<UserProfile | null> {
  try {
    const r = await fetch("/api/v1/me")
    if (!r.ok) return null
    return await r.json()
  } catch {
    return null
  }
}
