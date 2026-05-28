const fs = require('fs')
const path = require('path')
const { repoRoot, getScopedFiles } = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']

function normalizeCopy(value) {
  return String(value || '')
    .replace(/\{\{[^}]+\}\}/g, '')
    .replace(/[\/\\()[\]（）【】\s:：、，,。.!！?？-]/g, '')
    .trim()
}

function getAttribute(tag, name) {
  const pattern = new RegExp(`${name}\\s*=\\s*"([^"]*)"`)
  const match = tag.match(pattern)
  return match ? match[1].trim() : ''
}

function isPlaceholderLabelDrift(label, placeholder) {
  const normalizedLabel = normalizeCopy(label)
  const normalizedPlaceholder = normalizeCopy(placeholder)

  if (!normalizedLabel || !normalizedPlaceholder) {
    return false
  }

  if (!/^(请输入|请选择)/.test(normalizedPlaceholder)) {
    return false
  }

  const placeholderCore = normalizedPlaceholder.replace(/^(请输入|请选择)/, '')
  return !!placeholderCore && (
    normalizedLabel.includes(placeholderCore) ||
    placeholderCore.includes(normalizedLabel)
  )
}

function getLineNumber(content, index) {
  return content.slice(0, index).split(/\r?\n/).length
}

function findFailures(relativePath) {
  const absolutePath = path.join(repoRoot, relativePath)
  const content = fs.readFileSync(absolutePath, 'utf8')
  const failures = []
  const inputPattern = /<t-input\b[\s\S]*?\/?>/g

  for (const match of content.matchAll(inputPattern)) {
    const tag = match[0]
    const label = getAttribute(tag, 'label')
    const placeholder = getAttribute(tag, 'placeholder')

    if (isPlaceholderLabelDrift(label, placeholder)) {
      failures.push({
        line: getLineNumber(content, match.index || 0),
        label,
        placeholder
      })
    }
  }

  return failures
}

function main() {
  const files = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.wxml'] })
  const failures = []

  for (const relativePath of files) {
    const fileFailures = findFailures(relativePath)
    if (fileFailures.length) {
      failures.push({ relativePath, fileFailures })
    }
  }

  if (failures.length) {
    console.error('Placeholder label drift check failed. Visible labels own field purpose; placeholders must add format, constraint, example, or state-specific hints.')
    console.error('')
    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const item of failure.fileFailures) {
        console.error(`  - L${item.line}: label "${item.label}" repeats placeholder "${item.placeholder}"`)
      }
    }
    process.exit(1)
  }

  console.log(`check-placeholder-label-drift: validated ${files.length} WXML file(s)`)
}

main()
