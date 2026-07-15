import type { ImportAnalysisResult, ImportedChapter } from '../../domain/types'

const heading = /^(?:#{1,3}\s*)?(?:第\s*[0-9零一二三四五六七八九十百千两]+\s*[章节回卷]|chapter\s+\d+)\s*[:：、.\-]?\s*(.*)$/i

export function splitImportedChapters(text: string): ImportedChapter[] {
  const lines = text.replace(/\r\n/g, '\n').split('\n')
  const chapters: ImportedChapter[] = []
  let current: ImportedChapter | null = null
  for (const line of lines) {
    const match = line.trim().match(heading)
    if (match) {
      if (current) chapters.push({ ...current, content: current.content.trim() })
      current = { title: line.replace(/^#{1,3}\s*/, '').trim(), content: '' }
    } else if (current) current.content += `${line}\n`
  }
  if (current) chapters.push({ ...current, content: current.content.trim() })
  if (!chapters.length && text.trim()) return [{ title: '导入正文', content: text.trim() }]
  return chapters.filter((chapter) => chapter.content || chapter.title)
}

export function analyzeImportedText(chapters: ImportedChapter[]): ImportAnalysisResult {
  const excerpt = chapters[0]?.content.slice(0, 80) || '导入文本未提供可用证据'
  const names = [...new Set((chapters.map((item) => item.content).join('').match(/[\u4e00-\u9fa5]{2,3}/g) || []).filter((item) => !['收到', '一个', '正文', '来信'].includes(item)).slice(0, 2))]
  const firstName = names[0] || '待确认主角'
  const secondName = names[1] || '待确认配角'
  return {
    characters: [firstName, secondName].map((name, index) => ({ id: `import-char-${index}`, name, role: index ? 'supporting' : 'protagonist', summary: '由导入文本模拟识别，等待作者确认。', personality: [], motivation: '待确认', secrets: [], currentStatus: '已导入', factionIds: [], targetShare: index ? 30 : 50, arc: [], knowledge: [] })),
    factions: [{ id: 'import-faction-0', name: '待确认势力', goal: '从导入文本继续分析', resources: [], stance: '未知', memberIds: [] }],
    locations: [{ id: 'import-location-0', name: chapters[0]?.title || '待确认地点', summary: '模拟提取的故事地点', region: '未分类', x: 500, y: 350, visitChapterIds: [], eventIds: [] }],
    relationships: [{ id: 'import-rel-0', sourceId: 'import-char-0', targetId: 'import-char-1', type: '关系待确认', strength: 50, publicState: '未知', trueState: '未知', startChapter: 1 }],
    facts: [{ id: 'import-fact-0', text: `${firstName}出现在导入正文中`, category: 'imported', status: 'ai-inferred', confidence: 0.55, evidence: [{ chapterId: 'import-chapter-1', quote: excerpt }], scopeEntityIds: ['import-char-0'] }],
    events: [{ id: 'import-event-0', title: chapters[0]?.title || '导入事件', summary: excerpt, cause: '待确认', result: '待确认', storyTime: '时间待确认', narrativeOrder: 1, status: 'uncertain', participantIds: ['import-char-0'], factionIds: [], locationId: 'import-location-0', chapterIds: ['import-chapter-1'] }],
    outlineNodes: [{ id: 'import-outline-book', parentId: null, level: 'book', title: '导入作品反推大纲', summary: chapters.map((item) => item.title).join('、'), order: 1, status: 'active', linkedEntityIds: ['import-char-0'] }],
  }
}
