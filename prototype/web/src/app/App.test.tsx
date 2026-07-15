import { render, screen } from '@testing-library/react'
import { App } from './App'

test('renders the product name', () => {
  render(<App />)
  expect(screen.getByRole('heading', { name: '织境 Story Studio' })).toBeVisible()
})

test('renders the approved brand icon', () => {
  render(<App />)
  expect(screen.getByAltText('织境')).toHaveAttribute('src', '/brand/app-icon-64.png')
})
