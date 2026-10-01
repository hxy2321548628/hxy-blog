export interface MarkdownInsertion {
  value: string
  cursor: number
}

export function insertMarkdownImage(
  source: string,
  selectionStart: number,
  selectionEnd: number,
  altText: string,
  url: string,
): MarkdownInsertion {
  const start = Math.max(0, Math.min(source.length, selectionStart))
  const end = Math.max(start, Math.min(source.length, selectionEnd))
  const escapedAlt = altText.replaceAll('\\', '\\\\').replaceAll(']', '\\]')
  const markdown = `![${escapedAlt}](${url})`
  const before = source.slice(0, start)
  const after = source.slice(end)
  const leadingBreak = before.length > 0 && !before.endsWith('\n') ? '\n\n' : ''
  const trailingBreak = after.length > 0 && !after.startsWith('\n') ? '\n\n' : ''
  const inserted = `${leadingBreak}${markdown}${trailingBreak}`

  return {
    value: `${before}${inserted}${after}`,
    cursor: before.length + leadingBreak.length + markdown.length,
  }
}
