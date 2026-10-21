import { useEffect, useState } from 'react'

/** Wartość, która nadąża za `value` z opóźnieniem (np. wyszukiwanie przy pisaniu). */
export function useDebouncedValue<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}
