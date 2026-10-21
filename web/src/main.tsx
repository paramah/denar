import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/App.tsx'
import { installInvalidFocus } from './lib/focusInvalid.ts'
import './styles/fonts.css'
import './styles/tokens.css'
import './styles/global.css'

const container = document.getElementById('root')
if (!container) {
  throw new Error('index.html has no #root element')
}

installInvalidFocus()

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
