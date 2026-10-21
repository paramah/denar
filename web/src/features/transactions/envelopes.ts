import { features } from '../../lib/features.ts'

export type EnvelopeOption = { value: string; label: string }

/**
 * Koperty do wyboru w formularzach. Punkt rozszerzenia części C: gdy etap
 * 003 doda API kopert, ten hook czyta je (np. useEnvelopes(hid)) i zwraca
 * opcje; formularze się nie zmieniają. Dziś — pusta lista.
 */
export function useEnvelopeOptions(hid: string): EnvelopeOption[] {
  void hid
  return []
}

/** Czy formularze pokazują koperty i splity. */
export function envelopesEnabled(): boolean {
  return features.envelopes
}
