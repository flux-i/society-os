import { test, expect } from '@playwright/test'
import { money } from '../src/maintenance'

test('shared currency display preserves exact integer paise through both supported bounds', () => {
  for (const [paise, expected] of [
    [0, '₹0.00'], [-0, '₹0.00'], [1, '₹0.01'], [-1, '-₹0.01'],
    [1001, '₹10.01'], [125000, '₹1,250.00'], [124999, '₹1,249.99'],
    [9007199254740991, '₹9,00,71,99,25,47,409.91'],
    [-9007199254740991, '-₹9,00,71,99,25,47,409.91'],
  ] as const) expect(money(paise)).toBe(expected)
  for (const invalid of [0.5, NaN, Infinity, 9007199254740992, -9007199254740992]) expect(() => money(invalid)).toThrow(RangeError)
})
