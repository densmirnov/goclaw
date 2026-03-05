import { create } from "zustand";
import { LOCAL_STORAGE_KEYS } from "@/lib/constants";

interface AuthState {
  token: string;
  userId: string;
  senderID: string; // browser pairing: persistent device identity
  role: "admin" | "operator" | "viewer";
  connected: boolean;
  serverInfo: { name?: string; version?: string } | null;

  setCredentials: (token: string, userId: string) => void;
  setPairing: (senderID: string, userId: string) => void;
  setRole: (role: "admin" | "operator" | "viewer") => void;
  setConnected: (
    connected: boolean,
    serverInfo?: { name?: string; version?: string },
  ) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  token: localStorage.getItem(LOCAL_STORAGE_KEYS.TOKEN) ?? "",
  userId: localStorage.getItem(LOCAL_STORAGE_KEYS.USER_ID) ?? "",
  senderID: localStorage.getItem(LOCAL_STORAGE_KEYS.SENDER_ID) ?? "",
  role: "viewer",
  connected: false,
  serverInfo: null,

  setCredentials: (token, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.TOKEN, token);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ token, userId });
  },

  setPairing: (senderID, userId) => {
    localStorage.setItem(LOCAL_STORAGE_KEYS.SENDER_ID, senderID);
    localStorage.setItem(LOCAL_STORAGE_KEYS.USER_ID, userId);
    set({ senderID, userId });
  },
  setRole: (role) => {
    set({ role });
  },

  setConnected: (connected, serverInfo) => {
    set({ connected, serverInfo: serverInfo ?? null });
  },

  logout: () => {
    localStorage.removeItem(LOCAL_STORAGE_KEYS.TOKEN);
    localStorage.removeItem(LOCAL_STORAGE_KEYS.USER_ID);
    localStorage.removeItem(LOCAL_STORAGE_KEYS.SENDER_ID);
    set({
      token: "",
      userId: "",
      senderID: "",
      role: "viewer",
      connected: false,
      serverInfo: null,
    });
  },
}));
