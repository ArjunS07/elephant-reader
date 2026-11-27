import type { ReactNode } from 'react'

type IconProps = { size?: number }

function Glyph({ size = 16, children }: IconProps & { children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      aria-hidden="true"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.25"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {children}
    </svg>
  )
}

export function IconBookmark({ size }: IconProps) {
  return (
    <Glyph size={size}>
      <path d="M4.25 2.75h7.5v10.5L8 10.4 4.25 13.25V2.75z" />
    </Glyph>
  )
}

export function IconCheck({ size }: IconProps) {
  return (
    <Glyph size={size}>
      <path d="M3.5 8.25 6.6 11.4 12.5 4.6" />
    </Glyph>
  )
}

export function IconClose({ size }: IconProps) {
  return (
    <Glyph size={size}>
      <path d="M4 4.5 12 12.5M12 4.5 4 12.5" />
    </Glyph>
  )
}

export function IconExternal({ size = 13 }: IconProps) {
  return (
    <Glyph size={size}>
      <path d="M6 4.5H4.5V11.5H11.5V10" />
      <path d="M8 4.5h3.5V8" />
      <path d="M11.5 4.5 7 9" />
    </Glyph>
  )
}

export function IconDiscussion({ size }: IconProps) {
  return (
    <Glyph size={size}>
      <path d="M3.25 3.75h9.5v6.25H6.4L3.25 12.4V3.75z" />
    </Glyph>
  )
}
