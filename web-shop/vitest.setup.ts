import "@testing-library/jest-dom/vitest";

// Node 26 ships an experimental localStorage that needs `--localstorage-file`
// to function — when absent, getItem/setItem are undefined. Override with a
// Map-backed polyfill so test code (including module-load-time storage
// captures like Zustand's `createJSONStorage(() => localStorage)`) works.
if (
  typeof globalThis.localStorage === "undefined" ||
  typeof globalThis.localStorage.setItem !== "function"
) {
  const store = new Map<string, string>();
  Object.defineProperty(globalThis, "localStorage", {
    configurable: true,
    value: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, String(v));
      },
      removeItem: (k: string) => {
        store.delete(k);
      },
      clear: () => store.clear(),
      key: (i: number) => Array.from(store.keys())[i] ?? null,
      get length() {
        return store.size;
      },
    },
  });
}
