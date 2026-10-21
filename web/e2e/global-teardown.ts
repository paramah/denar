import { rmSync } from 'node:fs'
import { runDir } from './support/state.ts'

// Baza przebiegu znika razem ze swoim katalogiem.
export default function globalTeardown() {
  rmSync(runDir(), { recursive: true, force: true })
}
