import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Money } from './Money.tsx'

const text = (el: HTMLElement) => el.textContent?.replace(/[\u00a0\u202f]/g, ' ')

describe('Money', () => {
  it('shows an expense with a minus sign and the outflow tone', () => {
    render(<Money amount="-1234.5" currency="PLN" tone="auto" />)
    const el = screen.getByText(/zł/)
    expect(text(el)).toBe('−1234,50 zł')
    expect(el).toHaveAttribute('data-tone', 'outflow')
  })

  it('shows an inflow with a plus sign', () => {
    render(<Money amount="250" currency="EUR" tone="auto" />)
    const el = screen.getByText(/€/)
    expect(text(el)).toBe('+250,00 €')
    expect(el).toHaveAttribute('data-tone', 'inflow')
  })

  it('is neutral and unsigned by default', () => {
    render(<Money amount="12.30" currency="PLN" />)
    const el = screen.getByText(/zł/)
    expect(text(el)).toBe('12,30 zł')
    expect(el).toHaveAttribute('data-tone', 'neutral')
  })

  it('shows a dash with the raw value as title for a malformed amount', () => {
    render(<Money amount="12,3" currency="PLN" />)
    expect(screen.getByText('—')).toHaveAttribute('title', '12,3 PLN')
  })
})
