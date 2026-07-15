import { BookCopy, BookOpenText, Bot, Clock3, Database, GitBranch, Home, Map, Network, Route, ScrollText, Settings, ShieldCheck, Users } from 'lucide-react'

export const projectNavigation = [
  ['dashboard', '创作驾驶舱', Home], ['manuscript', '正文与场景', BookOpenText], ['outline', '故事架构', BookCopy], ['characters', '角色与势力', Users], ['world', '世界设定', Map], ['canon', '事实与知识', Database], ['timeline', '时间线', Clock3], ['graph', '关系拓扑', Network], ['map', '世界地图', Route], ['agents', '模型与智能体', Bot], ['workflow', '智能体工作流', Bot], ['review', '审稿中心', ShieldCheck], ['runs', '运行记录', ScrollText], ['versions', '版本历史', GitBranch], ['settings', '作品设置', Settings],
] as const

export type PageId = typeof projectNavigation[number][0] | 'library'
