import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/errors.ts'
import { DataList } from './DataList/DataList.tsx'
import { ConfirmDialog, Dialog } from './Dialog/Dialog.tsx'
import { EmptyState } from './EmptyState/EmptyState.tsx'
import { TextField } from './Field/Field.tsx'
import { FormAlert } from './Form/Form.tsx'
import { ToastProvider, useToast } from './Toast/Toast.tsx'

describe('Dialog', () => {
  function Harness() {
    const [open, setOpen] = useState(false)
    return (
      <>
        <button onClick={() => setOpen(true)}>Otwórz</button>
        <Dialog open={open} title="Okno" onClose={() => setOpen(false)}>
          <input aria-label="Pole" />
        </Dialog>
      </>
    )
  }

  it('focuses the content, closes on Escape and returns focus to the opener', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    await user.click(screen.getByRole('button', { name: 'Otwórz' }))
    expect(screen.getByRole('dialog', { name: 'Okno' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Pole' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Otwórz' })).toHaveFocus()
  })
})

describe('ConfirmDialog', () => {
  it('confirms only after the phrase is typed', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    render(
      <ConfirmDialog open title="Usunąć?" confirmLabel="Usuń" requirePhrase="Dom" onConfirm={onConfirm} onCancel={vi.fn()}>
        <p>Na zawsze.</p>
      </ConfirmDialog>,
    )
    const confirm = screen.getByRole('button', { name: 'Usuń' })
    expect(confirm).toBeDisabled()
    await user.type(screen.getByRole('textbox'), 'Dom')
    await user.click(confirm)
    expect(onConfirm).toHaveBeenCalledOnce()
  })
})

describe('Toast', () => {
  function Trigger({ kind }: { kind?: 'success' | 'error' }) {
    const toast = useToast()
    return <button onClick={() => toast.show({ message: 'Zapisano.', kind })}>Pokaż</button>
  }

  it('announces a message in the status region and hides it after a while', async () => {
    vi.useFakeTimers()
    try {
      render(
        <ToastProvider>
          <Trigger />
        </ToastProvider>,
      )
      act(() => screen.getByRole('button', { name: 'Pokaż' }).click())
      expect(screen.getByRole('status')).toHaveTextContent('Zapisano.')
      act(() => vi.advanceTimersByTime(6000))
      expect(screen.getByRole('status')).not.toHaveTextContent('Zapisano.')
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps an error until it is closed', async () => {
    const user = userEvent.setup()
    render(
      <ToastProvider>
        <Trigger kind="error" />
      </ToastProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Pokaż' }))
    await user.click(screen.getByRole('button', { name: 'Zamknij komunikat' }))
    expect(screen.getByRole('status')).not.toHaveTextContent('Zapisano.')
  })
})

describe('form pieces', () => {
  it('ties a field to its hint and error', () => {
    render(<TextField label="Nazwa" hint="Krótko." error="Za długa." />)
    const input = screen.getByRole('textbox', { name: 'Nazwa' })
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAccessibleDescription('Za długa. Krótko.')
  })

  it('lists field errors the form does not show itself', () => {
    const error = new ApiError({
      status: 422,
      code: 'validation',
      title: 'Błąd',
      errors: [
        { field: 'name', message: 'za długa' },
        { field: 'scopes', message: 'nieznany scope' },
      ],
    })
    render(<FormAlert error={error} shownFields={['name']} />)
    expect(screen.getByRole('alert')).toHaveTextContent('uprawnienia: nieznany scope')
    expect(screen.getByRole('alert')).not.toHaveTextContent('za długa')
  })
})

describe('DataList', () => {
  it('shows the empty state, then items and a load-more button', async () => {
    const user = userEvent.setup()
    const onLoadMore = vi.fn()
    const { rerender } = render(
      <DataList
        label="Lista"
        items={[] as string[]}
        getKey={(s) => s}
        renderItem={(s) => s}
        empty={<EmptyState icon="info" title="Pusto" />}
      />,
    )
    expect(screen.getByRole('heading', { name: 'Pusto' })).toBeInTheDocument()
    rerender(
      <DataList label="Lista" items={['a', 'b']} getKey={(s) => s} renderItem={(s) => s} hasMore onLoadMore={onLoadMore} />,
    )
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    await user.click(screen.getByRole('button', { name: 'Pokaż więcej' }))
    expect(onLoadMore).toHaveBeenCalled()
  })
})
