import { create } from "zustand";
import { events } from "../lib/transport";

type WindowState = {
  maximized: boolean;
  focused: boolean;
  subscribe: () => () => void;
};

export const useWindowStore = create<WindowState>((set) => ({
  maximized: false,
  focused: true,
  subscribe: () => {
    const off = events.windowState.on((state) => {
      set({ maximized: state.maximized, focused: state.focused });
    });
    return () => {
      off();
    };
  },
}));
