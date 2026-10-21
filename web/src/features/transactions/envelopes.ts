import { useMemo } from 'react'
import { useEnvelopeGroups, useEnvelopes } from '../../api/hooks/budget.ts'
import type { Envelope, EnvelopeGroup } from '../../api/types.ts'
import { features } from '../../lib/features.ts'

export type EnvelopeOption = { value: string; label: string }

/** „Codzienne · Jedzenie”, ukryte z dopiskiem; w kolejności grup i kopert. */
export function envelopeOptions(envelopes: Envelope[], groups: EnvelopeGroup[]): EnvelopeOption[] {
  const order = new Map(groups.map((group, index) => [group.id, index]))
  const names = new Map(groups.map((group) => [group.id, group.name]))
  return [...envelopes]
    .map((envelope, index) => ({ envelope, index }))
    .sort(
      (a, b) =>
        (order.get(a.envelope.group_id) ?? Infinity) - (order.get(b.envelope.group_id) ?? Infinity) || a.index - b.index,
    )
    .map(({ envelope }) => {
      const group = names.get(envelope.group_id)
      return {
        value: envelope.id,
        label: `${group ? `${group} · ` : ''}${envelope.name}${envelope.hidden ? ' (ukryta)' : ''}`,
      }
    })
}

/**
 * Koperty do wyboru w formularzach (transakcja, split, przelew, domyślna
 * koperta kontrahenta) — z API kopert, także ukryte (stara transakcja może
 * wskazywać ukrytą kopertę).
 */
export function useEnvelopeOptions(hid: string): EnvelopeOption[] {
  const enabled = features.envelopes
  const envelopes = useEnvelopes(hid, { enabled })
  const groups = useEnvelopeGroups(hid, { enabled })
  return useMemo(
    () => (enabled ? envelopeOptions(envelopes.data ?? [], groups.data ?? []) : []),
    [enabled, envelopes.data, groups.data],
  )
}

/** Nazwy kopert po id (wiersze rejestru, kontrahenci). */
export function useEnvelopeNames(hid: string): Map<string, string> {
  const enabled = features.envelopes
  const envelopes = useEnvelopes(hid, { enabled })
  return useMemo(() => new Map((envelopes.data ?? []).map((e) => [e.id, e.name])), [envelopes.data])
}

/** Czy formularze pokazują koperty i splity. */
export function envelopesEnabled(): boolean {
  return features.envelopes
}
