// jsdom has no showModal() and close(). This stand-in keeps the parts the
// components rely on: the `open` attribute, the `close` event, and Escape
// raising a cancelable `cancel` event the way a browser does for a modal.
// Focus trapping and the top layer stay untested here; e2e covers them.
export function installDialogShim() {
  const proto = window.HTMLDialogElement.prototype
  if (typeof proto.showModal === 'function') {
    return
  }
  proto.showModal = function showModal(this: HTMLDialogElement) {
    this.setAttribute('open', '')
  }
  proto.close = function close(this: HTMLDialogElement) {
    if (this.hasAttribute('open')) {
      this.removeAttribute('open')
      this.dispatchEvent(new Event('close'))
    }
  }
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape' || event.defaultPrevented) {
      return
    }
    const open = [...document.querySelectorAll('dialog[open]')].pop()
    open?.dispatchEvent(new Event('cancel', { cancelable: true }))
  })
}
