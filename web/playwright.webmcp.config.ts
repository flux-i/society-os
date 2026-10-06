import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests', testMatch: 'webmcp*.spec.ts', workers: 1,
  outputDir: process.env.SOCIETY_BROWSER_ARTIFACTS ?? '../reports/local/webmcp-test-results',
  reporter: 'list',
  use: {
    baseURL: process.env.SOCIETY_BROWSER_URL ?? 'http://127.0.0.1:8080',
    channel: 'chrome',
    launchOptions: { args: ['--enable-features=WebMCP,WebMCPTesting'] },
    screenshot: 'only-on-failure', trace: 'retain-on-failure',
  },
})
