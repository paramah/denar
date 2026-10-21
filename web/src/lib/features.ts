/**
 * Flagi funkcji zależnych od kolejnych etapów backendu. Obiekt modułu, żeby
 * testy mogły je przełączać; aplikacja ich nie zmienia w trakcie działania.
 *
 * envelopes — pola kopert i splity transakcji (API kopert z etapu 003,
 * włączone w części C1 etapu 007; zob. features/transactions/envelopes.ts).
 */
export const features = {
  envelopes: true,
}
