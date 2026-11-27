import { copyFileSync, mkdirSync } from 'node:fs'
import { dirname } from 'node:path'

const routes = ['404.html', 'feed/index.html', 'later/index.html', 'read/index.html', 'sources/index.html']

for (const route of routes) {
  const target = `dist/${route}`
  mkdirSync(dirname(target), { recursive: true })
  copyFileSync('dist/index.html', target)
}
