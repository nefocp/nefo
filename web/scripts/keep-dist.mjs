// Vite empties the output folder on every build. Put the placeholder back so
// `go build` (which embeds this folder) keeps working in a fresh checkout.
import { writeFileSync } from 'node:fs'

writeFileSync(new URL('../../internal/web/dist/.gitkeep', import.meta.url), '')
