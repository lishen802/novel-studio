import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.goto('/')
  await page.evaluate(() => localStorage.clear())
  await page.reload()
})

test('edits manuscript and keeps it after refresh', async ({ page }) => {
  await page.getByRole('button', { name: '正文与场景' }).click()
  await page.getByLabel('章节正文').fill('端到端测试：潮声越过警戒线。')
  await page.reload()
  await page.getByRole('button', { name: '正文与场景' }).click()
  await expect(page.getByLabel('章节正文')).toHaveValue('端到端测试：潮声越过警戒线。')
})

test('opens every primary story system', async ({ page }) => {
  for (const name of ['故事架构', '角色与势力', '世界设定', '事实与知识', '时间线', '关系拓扑', '世界地图']) {
    await page.getByRole('button', { name, exact: true }).click()
    await expect(page.getByRole('heading', { name: name === '故事架构' ? '五级故事架构' : name === '时间线' ? '多轨时间线' : name === '关系拓扑' ? '关系拓扑' : name === '世界地图' ? '世界地图与轨迹' : name })).toBeVisible()
  }
})
