const paths = {
  menu: 'M4 6h16 M4 12h16 M4 18h16',
  shield: 'M12 3 3 7v6c0 5 9 9 9 9s9-4 9-9V7z M8 12l3 3 5-6',
  key: 'M14 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M9 11v10 M9 16h4 M9 20h3',
  overview: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
  homes: 'M4 21V7l8-4 8 4v14 M2 21h20 M8 9h1 M15 9h1 M8 13h1 M15 13h1 M10 21v-4h4v4',
  records: 'M6 3h12v18H6z M9 7h6 M9 11h6 M9 15h4',
  receipt: 'M5 3h14v18l-3-2-4 2-4-2-3 2z M8 7h8 M8 11h8 M8 15h4',
  community: 'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2 M16 3a4 4 0 0 1 0 8 M22 21v-2a4 4 0 0 0-3-3.9 M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
  document: 'M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z M14 2v6h6 M8 13h8 M8 17h5',
  arrow: 'M7 17 17 7 M7 7h10v10',
  chevron: 'm9 5 7 7-7 7',
  down: 'm6 9 6 6 6-6',
  left: 'm15 5-7 7 7 7',
  search: 'M21 21l-5-5 M18 10.5a7.5 7.5 0 1 1-15 0 7.5 7.5 0 0 1 15 0',
  close: 'm6 6 12 12 M6 18 18 6',
  refresh: 'M20 7a9 9 0 1 0 1 9 M20 2v5h-5',
  leaf: 'M20 3C9 1 3 7 4 14s8 10 12 5 5-10 4-16Z M5 20 16 8',
  check: 'm5 12 4 4L19 6',
  spark: 'm12 3 2.4 6.6L21 12l-6.6 2.4L12 21l-2.4-6.6L3 12l6.6-2.4Z',
  globe: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M3 12h18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18',
} as const

export type IconName = keyof typeof paths

export function Icon({ name, className = '' }: { name: IconName; className?: string }) {
  return <svg aria-hidden="true" className={className} width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinecap="round" strokeLinejoin="round"><path d={paths[name]} /></svg>
}
