/**
 * Flagi funkcji zależnych od kolejnych etapów backendu. Obiekt modułu, żeby
 * testy mogły je przełączać; aplikacja ich nie zmienia w trakcie działania.
 *
 * envelopes — pola kopert i splity transakcji. Włącza je część C etapu 007,
 * gdy etap 003 doda API kopert (zob. features/transactions/envelopes.ts).
 */
export const features = {
  envelopes: false,
}
