import type { Page } from '@playwright/test'

export type ContextDocument = Document & { modelContext: {
  getTools: () => Promise<{ name: string }[]>
  executeTool: (tool: { name: string }, args: unknown, options?: { signal: AbortSignal }) => Promise<string>
} }
export async function names(page: Page) {
  return page.evaluate(async () => (await (document as ContextDocument).modelContext.getTools()).map(tool => tool.name))
}
export async function execute(page: Page, name: string, args: unknown) {
  return page.evaluate(async ({ name, args }) => {
    const context = (document as ContextDocument).modelContext
    const tool = (await context.getTools()).find(item => item.name === name)
    if (!tool) throw new Error('Expected registered native tool: ' + name)
    // Chrome 154 accepts JSON strings; Chrome 155 changed this to objects.
    const major = Number(navigator.userAgent.match(/Chrome\/(\d+)/)?.[1])
    return context.executeTool(tool, major < 155 ? JSON.stringify(args) : args)
  }, { name, args })
}
