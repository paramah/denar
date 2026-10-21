import { afterEach, describe, expect, it } from 'vitest'
import { installInvalidFocus } from './focusInvalid.ts'

const nextFrame = () => new Promise((resolve) => requestAnimationFrame(() => resolve(null)))

describe('installInvalidFocus', () => {
  let uninstall = () => {}
  afterEach(() => {
    uninstall()
    document.body.innerHTML = ''
  })

  it('focuses the first invalid field after a submit', async () => {
    uninstall = installInvalidFocus()
    document.body.innerHTML = `<form><input id="a"><input id="b"><button>Zapisz</button></form>`
    const form = document.querySelector('form')!
    form.addEventListener('submit', (event) => {
      event.preventDefault()
      document.getElementById('b')!.setAttribute('aria-invalid', 'true')
    })
    document.querySelector('button')!.focus()
    form.requestSubmit()
    await nextFrame()
    expect(document.activeElement?.id).toBe('b')
  })

  it('leaves the focus alone when the form is valid', async () => {
    uninstall = installInvalidFocus()
    document.body.innerHTML = `<form><input id="a"><button>Zapisz</button></form>`
    const form = document.querySelector('form')!
    form.addEventListener('submit', (event) => event.preventDefault())
    document.querySelector('button')!.focus()
    form.requestSubmit()
    await nextFrame()
    expect(document.activeElement?.tagName).toBe('BUTTON')
  })
})
