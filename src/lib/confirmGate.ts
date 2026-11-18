/** Two-step confirmation with a minimum gap (PRD FR-AI-PERM: 600ms) so a
 * double click cannot pass the destructive step. Pure for unit tests. */
export const CONFIRM_MIN_GAP_MS = 600;
export const CONFIRM_RESET_MS = 4000;

export type GateState = { armedAt: number | null };

export const initialGate: GateState = { armedAt: null };

/** Returns the next state and whether the action should fire now. */
export function advanceGate(
  state: GateState,
  now: number,
): { state: GateState; fire: boolean } {
  if (state.armedAt === null) {
    return { state: { armedAt: now }, fire: false };
  }
  if (now - state.armedAt < CONFIRM_MIN_GAP_MS) {
    return { state, fire: false };
  }
  return { state: initialGate, fire: true };
}

/** Drops an armed state that has waited too long. */
export function expireGate(state: GateState, now: number): GateState {
  if (state.armedAt !== null && now - state.armedAt > CONFIRM_RESET_MS) {
    return initialGate;
  }
  return state;
}
