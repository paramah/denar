/**
 * Po wysłaniu formularza z błędami walidacji przenosi focus na pierwsze
 * błędne pole (`aria-invalid="true"`), żeby komunikat nie zostawał poza
 * widokiem (np. w długim dialogu przewiniętym do przycisku „Zapisz”).
 * Nasłuch w fazie przechwytywania rusza przed handlerem Reacta; sprawdzenie
 * odkładamy na następną klatkę, gdy React już pokaże błędy.
 */
export function installInvalidFocus(root: Document = document): () => void {
  const onSubmit = (event: Event) => {
    const form = event.target
    if (!(form instanceof HTMLFormElement)) {
      return
    }
    requestAnimationFrame(() => {
      const active = root.activeElement
      if (active instanceof HTMLElement && active.getAttribute('aria-invalid') === 'true' && form.contains(active)) {
        return
      }
      form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
    })
  }
  root.addEventListener('submit', onSubmit, true)
  return () => root.removeEventListener('submit', onSubmit, true)
}
