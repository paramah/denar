// Drobne preferencje przeglądarki. Pamięć bywa zablokowana (tryb prywatny),
// więc każdy odczyt i zapis jest w try/catch, a aplikacja działa bez niej.

export const storageKeys = {
  theme: 'denar:theme',
  household: 'denar:household',
  /** sessionStorage: token zaproszenia między logowaniem a przyjęciem. */
  invite: 'denar:invite',
} as const

type Area = 'local' | 'session'

function area(which: Area): Storage | null {
  try {
    return which === 'local' ? window.localStorage : window.sessionStorage
  } catch {
    return null
  }
}

export function readStored(key: string, which: Area = 'local'): string | null {
  try {
    return area(which)?.getItem(key) ?? null
  } catch {
    return null
  }
}

export function writeStored(key: string, value: string | null, which: Area = 'local'): void {
  try {
    const store = area(which)
    if (value === null) {
      store?.removeItem(key)
    } else {
      store?.setItem(key, value)
    }
  } catch {
    // Bez pamięci preferencja nie przetrwa przeładowania; to wszystko.
  }
}
