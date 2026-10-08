import { createHash } from 'node:crypto'
import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { Plugin } from 'vite'

export function publicShell(): Plugin {
  const critical = new Set<string>(['/', '/manifest.webmanifest', '/icon.svg', '/icon-192.png', '/icon-512.png'])
  return {
    name: 'society-public-shell', apply: 'build', enforce: 'post',
    generateBundle(_, bundle) {
      const visit = (name: string) => {
        if (critical.has('/' + name)) return
        critical.add('/' + name)
        const item = bundle[name]
        if (item?.type !== 'chunk') return
        item.imports.forEach(visit)
        const metadata = item as typeof item & { viteMetadata?: { importedCss: Set<string> } }
        metadata.viteMetadata?.importedCss.forEach(file => critical.add('/' + file))
      }
      for (const item of Object.values(bundle)) if (item.type === 'chunk' && item.isEntry) visit(item.fileName)
    },
    closeBundle() {
      const root = resolve('../build/web')
      const walk = (folder: string): string[] => readdirSync(resolve(root, folder), { withFileTypes: true }).flatMap(entry => entry.isDirectory() ? walk(folder + entry.name + '/') : [folder + entry.name])
      const files = walk('').filter(file => file === 'index.html' || ['manifest.webmanifest', 'icon.svg', 'icon-192.png', 'icon-512.png'].includes(file) || /^assets\/[\w.-]+\.(js|css|woff2?)$/.test(file))
      const assets = Object.fromEntries(files.map(file => [file === 'index.html' ? '/' : '/' + file, createHash('sha256').update(readFileSync(resolve(root, file))).digest('hex')]))
      for (const file of files) if (file.endsWith('.woff2')) critical.add('/' + file)
      // Only initial static imports/CSS enter precache. Deferred screen code stays deferred.
      for (const path of critical) if (!assets[path]) throw new Error('Missing public shell dependency: ' + path)
      const workerSource = readFileSync(resolve('pwa-worker.js'), 'utf8')
      const version = createHash('sha256').update(JSON.stringify(assets)).update(workerSource).digest('hex').slice(0, 20)
      const definition = { version, assets, precache: [...critical] }
      writeFileSync(resolve(root, 'sw.js'), 'const STATIC = ' + JSON.stringify(definition) + ';\n' + workerSource)
    },
  }
}
