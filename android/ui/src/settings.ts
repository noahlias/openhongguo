// The app's settings, the desktop.json fields that make sense on a phone (desktop/src/settings.rs),
// kept in the app's storage. The signal stays private: pages read `settings()` and write `updateSettings`.
import { signal, Storage } from "nsh";

export interface Settings {
  /** "zh" or "en". */
  language: string;
  /** "auto", "dark" or "light". */
  theme: string;
  /** "best", "worst" or a definition such as "720p". */
  quality: string;
  autoplay: boolean;
}

export const defaults: Settings = { language: "zh", theme: "dark", quality: "best", autoplay: true };

const KEY = "settings";
const state = signal<Settings>(defaults);

export const settings = () => state();

/** Reads the stored settings; missing fields keep their defaults, a malformed file is an error. */
export async function loadSettings() {
  const raw = await Storage.getItem(KEY);
  if (raw !== null) state.set({ ...defaults, ...(JSON.parse(raw) as Partial<Settings>) });
}

export async function updateSettings(change: Partial<Settings>) {
  const next = { ...state(), ...change };
  state.set(next);
  await Storage.setItem(KEY, JSON.stringify(next));
}
