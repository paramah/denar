/**
 * Ścieżka powrotu po zalogowaniu: tylko adres tej aplikacji (zaczyna się
 * od jednego „/”, nie prowadzi do API ani na inny host). Inaczej „/”.
 */
export function safeReturnTo(value: string | null | undefined): string {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
    return '/'
  }
  if (/^\/(api|healthz|mcp)(\/|$|\?)/.test(value) || value.startsWith('/login')) {
    return '/'
  }
  return value
}

/** Adres logowania z powrotem na bieżącą stronę. */
export function loginUrl(location: { pathname: string; search: string }): string {
  const here = `${location.pathname}${location.search}`
  return here === '/' ? '/login' : `/login?return_to=${encodeURIComponent(safeReturnTo(here))}`
}
