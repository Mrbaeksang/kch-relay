import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'
import { App } from './App'

afterEach(cleanup)
test('개발 준비 화면은 거래 기능을 제공한다고 오해시키지 않는다', () => {
  render(<App />)
  expect(screen.getByRole('heading', { level: 1 }).textContent).toContain('다음 쓰임')
  expect(screen.getByRole('status').textContent).toContain('아직 상품 조회·가입·거래 기능은 제공하지 않습니다')
  expect(screen.queryByRole('button', { name: /로그인|신청/ })).toBeNull()
})
