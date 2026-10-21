import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import * as f from '../../test/msw/fixtures.ts'
import { server } from '../../test/msw/server.ts'
import { apiUrl } from '../../test/msw/url.ts'
import { renderApp } from '../../test/render.tsx'

describe('AuditTab', () => {
  it('pages with the cursor, names actors and filters', async () => {
    const requests: URLSearchParams[] = []
    server.use(
      http.get(apiUrl('/households/:hid/bots'), () => HttpResponse.json({ items: [f.bot()] })),
      http.get(apiUrl('/households/:hid/audit'), ({ request }) => {
        const params = new URL(request.url).searchParams
        requests.push(params)
        if (params.get('actor_kind') === 'bot') {
          return HttpResponse.json({
            items: [f.auditEntry(9, { actor: { kind: 'bot', id: 'b-claude' }, action: 'bot.token_rotated', resource_type: 'bot', changes: {} })],
            next_cursor: null,
          })
        }
        if (params.get('cursor') === 'c2') {
          return HttpResponse.json({ items: [f.auditEntry(2, { action: 'household.created', changes: {} })], next_cursor: null })
        }
        return HttpResponse.json({ items: [f.auditEntry(3)], next_cursor: 'c2' })
      }),
    )
    const { user } = renderApp('/h/h-dom/settings/audit')
    const list = await screen.findByRole('list', { name: 'Wpisy dziennika' })
    expect(within(list).getByText('Zmiana ustawień gospodarstwa')).toBeInTheDocument()
    expect(within(list).getByText('Ada Kowalska')).toBeInTheDocument()

    await user.click(screen.getByText('Zmiany (1)'))
    expect(within(list).getByText('Stary dom')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Pokaż starsze' }))
    expect(await within(list).findByText('Założenie gospodarstwa')).toBeInTheDocument()
    expect(requests.at(-1)?.get('cursor')).toBe('c2')
    expect(screen.queryByRole('button', { name: 'Pokaż starsze' })).not.toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: 'Kto' }), 'bot')
    expect(await screen.findByText('Nowy token bota')).toBeInTheDocument()
    expect(screen.getByText('Bot Claude')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('Założenie gospodarstwa')).not.toBeInTheDocument())
  })

  it('says when nothing matches the filters', async () => {
    const { user } = renderApp('/h/h-dom/settings/audit')
    expect(await screen.findByText('Tu pojawi się każda zmiana w gospodarstwie.')).toBeInTheDocument()
    await user.selectOptions(screen.getByRole('combobox', { name: 'Czego dotyczy' }), 'bot')
    expect(await screen.findByText('Żaden wpis nie pasuje do filtrów.')).toBeInTheDocument()
  })
})
