export function createOffsiteFixture(root: string, binary: string, initialMode?: string): Promise<{
  origin: string; config: string; reader: string; identities: string[]; close: () => Promise<void>
}>
