import { analyzeImportedText, splitImportedChapters } from './importText'

test('splits Chinese and Markdown chapter headings', () => {
  expect(splitImportedChapters('第1章 起风\n正文A\n第2章 入城\n正文B')).toHaveLength(2)
  expect(splitImportedChapters('# 第一章\n正文A\n# 第二章\n正文B')).toHaveLength(2)
})

test('uses one fallback chapter when no heading exists', () => {
  expect(splitImportedChapters('只有正文')[0].title).toBe('导入正文')
})

test('creates unconfirmed story candidates with evidence', () => {
  const chapters = splitImportedChapters('第一章 雾港\n沈砚在旧港收到许棠的来信。')
  const result = analyzeImportedText(chapters)
  expect(result.characters.length).toBeGreaterThan(0)
  expect(result.facts[0].status).toBe('ai-inferred')
  expect(result.facts[0].evidence[0].quote.length).toBeGreaterThan(0)
})
