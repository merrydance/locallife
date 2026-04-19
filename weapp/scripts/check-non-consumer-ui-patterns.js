const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries,
  getDiffBase,
  getScopedFiles,
  normalizeRelativePath,
  readFileAtRevision
} = require('./gate-utils')

const NON_CONSUMER_ROOTS = [
  'weapp/miniprogram/pages/merchant/',
  'weapp/miniprogram/pages/operator/',
  'weapp/miniprogram/pages/platform/',
  'weapp/miniprogram/pages/rider/'
]

const ALLOW_TEXT_ACTION_MARKER = 'weapp-gate allow-text-action:'
const ALLOW_EXPLANATORY_CARD_MARKER = 'weapp-gate allow-explanatory-card:'
const ALLOW_FIXED_FOOTER_MARKER = 'weapp-gate allow-fixed-footer-shell:'

const EXPLANATORY_CARD_CLASS_REGEX = /\b(?:notice-card|inline-note-card|guide-card|helper-card|intro-card|tips-card|explain-card|description-card)\b/
const FIXED_FOOTER_CLASS_REGEX = /\b(?:footer-bar|save-wrap|footer-actions|bottom-action|bottom-actions)\b/
const TEXT_ACTION_LABELS = new Set(['新增', '添加', '编辑', '删除', '移除', '测试', '状态'])
const SMALL_BUTTON_REGEX = /\bsize\s*=\s*(["'])(small|extra-small)\1/
const TEXT_VARIANT_REGEX = /\bvariant\s*=\s*(["'])text\1/
const ICON_ATTR_REGEX = /\bicon\s*=/

function buildLineCountMap(source) {
  const counts = new Map()

  for (const line of source.split('\n')) {
    const normalizedLine = line.trim()

    if (!normalizedLine) {
      continue
    }

    counts.set(normalizedLine, (counts.get(normalizedLine) || 0) + 1)
  }

  return counts
}

function buildBlockCountMap(blocks) {
  const counts = new Map()

  for (const block of blocks) {
    counts.set(block, (counts.get(block) || 0) + 1)
  }

  return counts
}

function consumeCount(counts, key) {
  if (!key) {
    return false
  }

  const remaining = counts.get(key) || 0

  if (remaining === 0) {
    return false
  }

  if (remaining === 1) {
    counts.delete(key)
  } else {
    counts.set(key, remaining - 1)
  }

  return true
}

function getTargetFiles(changedOnly) {
  if (!changedOnly) {
    return getScopedFiles({ roots: NON_CONSUMER_ROOTS, extensions: ['.wxml'] })
  }

  return Array.from(new Set(
    getChangedEntries()
      .map((entry) => normalizeRelativePath(entry.filePath))
      .filter((filePath) => NON_CONSUMER_ROOTS.some((root) => filePath.startsWith(root)))
      .filter((filePath) => filePath.endsWith('.wxml'))
  )).sort()
}

function hasNearbyMarker(lines, lineIndex, marker) {
  const start = Math.max(0, lineIndex - 2)

  for (let index = start; index <= lineIndex; index += 1) {
    if (lines[index].includes(marker)) {
      return true
    }
  }

  return false
}

function collectExplanatoryCardFailures(relativePath, lines, legacyLineCounts) {
  const failures = []

  lines.forEach((line, index) => {
    if (!EXPLANATORY_CARD_CLASS_REGEX.test(line)) {
      return
    }

    if (hasNearbyMarker(lines, index, ALLOW_EXPLANATORY_CARD_MARKER)) {
      return
    }

    if (consumeCount(legacyLineCounts, line.trim())) {
      return
    }

    failures.push({
      relativePath,
      lineNumber: index + 1,
      reason: 'non-consumer page uses a local explanatory-card pattern; push the guidance into labels, notes, state strips, or action-adjacent copy by default',
      lineText: line.trim(),
      marker: `Add ${ALLOW_EXPLANATORY_CARD_MARKER} <reason> only when the explanation itself is the task or warning surface.`
    })
  })

  return failures
}

function collectFixedFooterShellFailures(relativePath, lines, legacyLineCounts) {
  const failures = []

  lines.forEach((line, index) => {
    if (!FIXED_FOOTER_CLASS_REGEX.test(line)) {
      return
    }

    if (hasNearbyMarker(lines, index, ALLOW_FIXED_FOOTER_MARKER)) {
      return
    }

    if (consumeCount(legacyLineCounts, line.trim())) {
      return
    }

    failures.push({
      relativePath,
      lineNumber: index + 1,
      reason: 'non-consumer page introduces a local fixed-footer shell; default to content-flow form actions inside page-content and let page shell handle safe area',
      lineText: line.trim(),
      marker: `Add ${ALLOW_FIXED_FOOTER_MARKER} <reason> only when a persistent fixed action bar is genuinely required by the task.`
    })
  })

  return failures
}

function stripTags(text) {
  return text
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function normalizeBlock(text) {
  return text.replace(/\s+/g, ' ').trim()
}

function collectTextActionBlocks(content) {
  const blocks = []
  const buttonRegex = /<t-button\b([\s\S]*?)>([\s\S]*?)<\/t-button>/g
  let match = buttonRegex.exec(content)

  while (match) {
    const attrs = match[1] || ''
    const label = stripTags(match[2] || '')

    if (TEXT_ACTION_LABELS.has(label) && (TEXT_VARIANT_REGEX.test(attrs) || SMALL_BUTTON_REGEX.test(attrs)) && !ICON_ATTR_REGEX.test(attrs)) {
      blocks.push(normalizeBlock(match[0]))
    }

    match = buttonRegex.exec(content)
  }

  return blocks
}

function collectTextActionFailures(relativePath, content, lines, legacyBlockCounts) {
  const failures = []
  const buttonRegex = /<t-button\b([\s\S]*?)>([\s\S]*?)<\/t-button>/g
  let match = buttonRegex.exec(content)

  while (match) {
    const attrs = match[1] || ''
    const label = stripTags(match[2] || '')

    if (!TEXT_ACTION_LABELS.has(label)) {
      match = buttonRegex.exec(content)
      continue
    }

    const isLocalActionButton = TEXT_VARIANT_REGEX.test(attrs) || SMALL_BUTTON_REGEX.test(attrs)
    const hasIcon = ICON_ATTR_REGEX.test(attrs)

    if (!isLocalActionButton || hasIcon) {
      match = buttonRegex.exec(content)
      continue
    }

    const lineNumber = content.slice(0, match.index).split('\n').length

    if (hasNearbyMarker(lines, lineNumber - 1, ALLOW_TEXT_ACTION_MARKER)) {
      match = buttonRegex.exec(content)
      continue
    }

    if (consumeCount(legacyBlockCounts, normalizeBlock(match[0]))) {
      match = buttonRegex.exec(content)
      continue
    }

    failures.push({
      relativePath,
      lineNumber,
      reason: `text-only local action \"${label}\" detected on a non-consumer page; default to icon buttons or icon-led small buttons for local actions`,
      lineText: stripTags(match[0]),
      marker: `Add ${ALLOW_TEXT_ACTION_MARKER} <reason> only when a text-only action is genuinely clearer than an icon-led affordance.`
    })

    match = buttonRegex.exec(content)
  }

  return failures
}

function main() {
  const changedOnly = process.argv.includes('--changed-only')
  const diffBase = changedOnly ? getDiffBase() : null
  const targetFiles = getTargetFiles(changedOnly)

  if (targetFiles.length === 0) {
    console.log(`check-non-consumer-ui-patterns: no ${changedOnly ? 'changed' : 'scannable'} non-consumer Mini Program WXML files detected`)
    return
  }

  const failures = []

  for (const relativePath of targetFiles) {
    const absolutePath = path.join(repoRoot, relativePath)

    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const content = fs.readFileSync(absolutePath, 'utf8')
    const lines = content.split('\n')
    const legacyContent = changedOnly ? readFileAtRevision(diffBase, relativePath) : ''
    const legacyLineCounts = changedOnly ? buildLineCountMap(legacyContent) : new Map()
    const legacyBlockCounts = changedOnly ? buildBlockCountMap(collectTextActionBlocks(legacyContent)) : new Map()

    failures.push(...collectExplanatoryCardFailures(relativePath, lines, legacyLineCounts))
    failures.push(...collectFixedFooterShellFailures(relativePath, lines, legacyLineCounts))
    failures.push(...collectTextActionFailures(relativePath, content, lines, legacyBlockCounts))
  }

  if (failures.length > 0) {
    console.error('Non-consumer UI pattern gate failed. Merchant/operator/platform/rider pages must stay brief, direct, and TDesign-first on small screens.')
    console.error('Default policy: avoid explanatory-card blocks, avoid local fixed-footer shells, and use icon buttons or icon-led small buttons for local actions unless a narrow exception is documented.')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.relativePath}:${failure.lineNumber}`)
      console.error(`  - ${failure.reason}`)
      console.error(`  - ${failure.lineText}`)
      console.error(`  - ${failure.marker}`)
    }

    process.exit(1)
  }

  console.log(`check-non-consumer-ui-patterns: validated ${targetFiles.length} WXML file(s)`) 
}

main()