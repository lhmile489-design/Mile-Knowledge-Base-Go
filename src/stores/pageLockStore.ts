import { create } from "zustand";

/** While a skills dialog is open the title-bar page switch is locked: leaving
 * the page would unmount the dialog halfway through a write (M9; PRD Q-UX-6).
 * Each dialog takes one lock and gives it back when it unmounts.
 *
 * Boundary noted in the M9 T09 review: only the PageSwitch button reads this
 * store — `appStore.showPage` itself does not. Every programmatic caller was
 * audited in M9 and none can fire while a dialog is up (the in-page entries
 * sit inside the inert region). Any NEW `showPage` caller added later must
 * check `usePageLockStore.getState().locks` first or be unreachable from an
 * open dialog. */
interface PageLockState {
  locks: number;
  acquire: () => () => void;
}

export const usePageLockStore = create<PageLockState>((set) => ({
  locks: 0,
  acquire: () => {
    set((state) => ({ locks: state.locks + 1 }));
    let released = false;
    return () => {
      if (released) return;
      released = true;
      set((state) => ({ locks: Math.max(0, state.locks - 1) }));
    };
  },
}));
