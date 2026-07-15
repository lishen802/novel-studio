import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => { await page.goto('/'); await page.evaluate(() => localStorage.clear()); await page.reload() })

test('runs, pauses, resumes, and cancels the workflow', async ({ page }) => {
  await page.getByRole('button', { name: '智能体工作流' }).click()
  await page.getByRole('button', { name: '开始运行' }).click()
  await page.getByRole('button', { name: '暂停' }).click()
  await expect(page.getByRole('button', { name: '继续' })).toBeVisible()
  await page.getByRole('button', { name: '继续' }).click()
  await page.getByRole('button', { name: '取消' }).click()
  await expect(page.getByRole('button', { name: '开始运行' })).toBeVisible()
})

test('creates and restores a version snapshot', async ({ page }) => {
  await page.getByRole('button', { name: '版本历史' }).click()
  page.once('dialog', (dialog) => dialog.accept('发布前基线'))
  await page.getByRole('button', { name: '创建快照' }).click()
  await expect(page.getByText('发布前基线')).toBeVisible()
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: '恢复此版本' }).click()
  await expect(page.getByRole('button', { name: /^恢复前快照/ })).toBeVisible()
})
