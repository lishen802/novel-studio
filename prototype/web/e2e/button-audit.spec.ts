import { expect, test } from '@playwright/test'

test('all enabled buttons have an accessible name', async ({ page }) => {
  await page.goto('/')
  const buttons = page.getByRole('button')
  for (let index = 0; index < await buttons.count(); index += 1) {
    const button = buttons.nth(index)
    if (await button.isDisabled()) continue
    await expect(button).not.toHaveAccessibleName('')
  }
})

test('dashboard is usable at narrow width', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: '雾港来信' })).toBeVisible()
  await expect(page.getByRole('button', { name: '运行章节工作流' })).toBeVisible()
  await page.getByRole('button', { name: '切换侧栏' }).click()
  await expect.poll(async () => (await page.locator('.project-sidebar').boundingBox())?.x).toBeGreaterThanOrEqual(54)
  await expect(page.getByRole('button', { name: '正文与场景' })).toBeVisible()
})
