const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getGateScope,
  getScopedFiles,
  getDiffBase,
  readFileAtRevision
} = require('./gate-utils')

const SOURCE_ROOTS = [
  'weapp/miniprogram/pages/',
  'weapp/miniprogram/components/'
]

const ALLOW_NATIVE_CONTROL_MARKER = 'weapp-gate allow-native-control:'
const ALLOW_INTERNAL_CLASS_MARKER = 'weapp-gate allow-tdesign-internal-class:'

const NATIVE_CONTROL_RULES = [
  {
    name: 'native input',
    regex: /<input\b/i,
    allowedPattern: /type\s*=\s*(["'])nickname\1/i,
    guidance: 'use t-input or t-search unless the platform capability genuinely requires native input'
  },
  {
    name: 'native textarea',
    regex: /<textarea\b/i,
    guidance: 'use t-textarea unless the platform capability genuinely requires native textarea'
  },
  {
    name: 'native picker',
    regex: /<picker(?:-view|-view-column)?\b/i,
    guidance: 'use t-picker or a TDesign date-time picker before falling back to native picker controls'
  },
  {
    name: 'native switch',
    regex: /<switch\b/i,
    guidance: 'use t-switch unless the platform capability genuinely requires native switch'
  },
  {
    name: 'native checkbox',
    regex: /<checkbox(?:-group)?\b/i,
    guidance: 'use t-checkbox or t-checkbox-group before falling back to native checkbox controls'
  },
  {
    name: 'native radio',
    regex: /<radio(?:-group)?\b/i,
    guidance: 'use t-radio or t-radio-group before falling back to native radio controls'
  },
  {
    name: 'native slider',
    regex: /<slider\b/i,
    guidance: 'use the closest supported TDesign control or document the missing capability explicitly'
  },
  {
    name: 'native editor',
    regex: /<editor\b/i,
    guidance: 'use the closest supported TDesign composition or document the missing capability explicitly'
  }
]

function isScopedSource(filePath, extension) {
  return SOURCE_ROOTS.some((root) => filePath.startsWith(root)) && filePath.endsWith(extension)
}

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

function consumeLegacyLine(lineCounts, line) {
  const normalizedLine = line.trim()

  if (!normalizedLine) {
    return false
  }

  const remaining = lineCounts.get(normalizedLine) || 0

  if (remaining === 0) {
    return false
  }

  if (remaining === 1) {
    lineCounts.delete(normalizedLine)
  } else {
    lineCounts.set(normalizedLine, remaining - 1)
  }

  return true
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

function collectNativeControlFailures(relativePath, content, legacyLineCounts) {
  const failures = []
  const lines = content.split('\n')

  lines.forEach((line, index) => {
    const matchedRule = NATIVE_CONTROL_RULES.find((rule) => {
      if (!rule.regex.test(line)) {
        return false
      }

      if (rule.allowedPattern && rule.allowedPattern.test(line)) {
        return false
      }

      return true
    })

    if (!matchedRule) {
      return
    }

    if (hasNearbyMarker(lines, index, ALLOW_NATIVE_CONTROL_MARKER)) {
      return
    }

    if (consumeLegacyLine(legacyLineCounts, line)) {
      return
    }

    failures.push({
      relativePath,
      lineNumber: index + 1,
      reason: `${matchedRule.name} is outside the default TDesign boundary; ${matchedRule.guidance}`,
      lineText: line.trim(),
      marker: `Add ${ALLOW_NATIVE_CONTROL_MARKER} <reason> above the control only when the exception is truly platform-required.`
    })
  })

  return failures
}

function collectInternalClassFailures(relativePath, content, legacyLineCounts) {
  const failures = []
  const lines = content.split('\n')

  lines.forEach((line, index) => {
    if (!/\.t-[a-z0-9_-]+/i.test(line)) {
      return
    }

    if (hasNearbyMarker(lines, index, ALLOW_INTERNAL_CLASS_MARKER)) {
      return
    }

    if (consumeLegacyLine(legacyLineCounts, line)) {
      return
    }

    failures.push({
      relativePath,
      lineNumber: index + 1,
      reason: 'direct TDesign internal class selector detected; prefer documented props, slots, external classes, and custom wrapper classes',
      lineText: line.trim(),
      marker: `Add ${ALLOW_INTERNAL_CLASS_MARKER} <reason> above the selector only for a tightly scoped, documented compatibility exception.`
    })
  })

  return failures
}

function main() {
  const scope = getGateScope()
  const diffBase = scope === 'changed' ? getDiffBase() : null
  const changedWxmlFiles = getScopedFiles({ roots: SOURCE_ROOTS, extensions: ['.wxml'] })
    .filter((filePath) => isScopedSource(filePath, '.wxml'))
  const changedWxssFiles = getScopedFiles({ roots: SOURCE_ROOTS, extensions: ['.wxss'] })
    .filter((filePath) => isScopedSource(filePath, '.wxss'))

  if (changedWxmlFiles.length === 0 && changedWxssFiles.length === 0) {
    console.log(`check-tdesign-boundary: no ${scope === 'changed' ? 'changed' : 'scannable'} Mini Program page or component view files detected`)
    return
  }

  const failures = []

  for (const relativePath of changedWxmlFiles) {
    const absolutePath = path.join(repoRoot, relativePath)

    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const content = fs.readFileSync(absolutePath, 'utf8')
    const legacyLineCounts = scope === 'changed' ? buildLineCountMap(readFileAtRevision(diffBase, relativePath)) : new Map()
    failures.push(...collectNativeControlFailures(relativePath, content, legacyLineCounts))
  }

  for (const relativePath of changedWxssFiles) {
    const absolutePath = path.join(repoRoot, relativePath)

    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const content = fs.readFileSync(absolutePath, 'utf8')
    const legacyLineCounts = scope === 'changed' ? buildLineCountMap(readFileAtRevision(diffBase, relativePath)) : new Map()
    failures.push(...collectInternalClassFailures(relativePath, content, legacyLineCounts))
  }

  if (failures.length > 0) {
    console.error('TDesign boundary gate failed. Changed Mini Program pages and components must stay inside supported TDesign usage by default.')
    console.error('Use TDesign components first. If the platform capability genuinely forces an exception, add the explicit gate marker with a concrete reason.')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.relativePath}:${failure.lineNumber}`)
      console.error(`  - ${failure.reason}`)
      console.error(`  - ${failure.lineText}`)
      console.error(`  - ${failure.marker}`)
    }

    process.exit(1)
  }

  console.log(`check-tdesign-boundary: validated ${changedWxmlFiles.length} WXML file(s) and ${changedWxssFiles.length} WXSS file(s)`) 
}

main()