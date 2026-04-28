const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries,
  getDiffBase,
  normalizeRelativePath,
  readFileAtRevision
} = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const SCRIPT_EXTENSIONS = new Set(['.ts', '.js'])
const WXML_EXTENSION = '.wxml'
const API_IMPORT_REGEX = /(?:from\s+['"](?:@\/|(?:\.\.\/)+)api\/([^/'"]+)|require\(\s*['"](?:@\/|(?:\.\.\/)+)api\/([^/'"]+))/g
const EXPLANATORY_SURFACE_REGEX = /\b(?:notice-card|inline-note-card|guide-card|helper-card|intro-card|tips-card|explain-card|description-card)\b/
const ALLOW_PAGE_API_COMPOSITION = 'weapp-gate allow-page-api-composition:'
const ALLOW_EXPLANATORY_SURFACE = 'weapp-gate allow-explanatory-surface:'

function isSurfaceFile(filePath) {
  return SURFACE_ROOTS.some((root) => filePath.startsWith(root))
}

function collectApiDomains(content) {
  const domains = new Set()
  let match = API_IMPORT_REGEX.exec(content)

  while (match) {
    const domain = match[1] || match[2]

    if (domain) {
      domains.add(domain)
    }

    match = API_IMPORT_REGEX.exec(content)
  }

  return Array.from(domains).sort()
}

function hasNewApiComposition(currentDomains, legacyDomains) {
  if (currentDomains.length <= 1) {
    return false
  }

  if (legacyDomains.length <= 1) {
    return true
  }

  const legacyDomainSet = new Set(legacyDomains)
  return currentDomains.some((domain) => !legacyDomainSet.has(domain))
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

function consumeCount(counts, key) {
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

function hasNearbyMarker(lines, lineIndex, marker) {
  const start = Math.max(0, lineIndex - 2)

  for (let index = start; index <= lineIndex; index += 1) {
    if (lines[index].includes(marker)) {
      return true
    }
  }

  return false
}

function getChangedSurfaceFiles() {
  return Array.from(new Set(
    getChangedEntries()
      .map((entry) => normalizeRelativePath(entry.filePath))
      .filter(isSurfaceFile)
      .filter((filePath) => SCRIPT_EXTENSIONS.has(path.extname(filePath)) || path.extname(filePath) === WXML_EXTENSION)
  )).sort()
}

function main() {
  const diffBase = getDiffBase()
  const targetFiles = getChangedSurfaceFiles()

  if (targetFiles.length === 0) {
    console.log('check-frontend-architecture-boundary: no changed Mini Program page/component files detected')
    return
  }

  const failures = []

  for (const relativePath of targetFiles) {
    const absolutePath = path.join(repoRoot, relativePath)

    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const content = fs.readFileSync(absolutePath, 'utf8')
    const legacyContent = readFileAtRevision(diffBase, relativePath)
    const extension = path.extname(relativePath)
    const fileFailures = []

    if (SCRIPT_EXTENSIONS.has(extension)) {
      const currentDomains = collectApiDomains(content)
      const legacyDomains = collectApiDomains(legacyContent)

      if (hasNewApiComposition(currentDomains, legacyDomains) && !content.includes(ALLOW_PAGE_API_COMPOSITION)) {
        fileFailures.push(
          `page/component now composes multiple API domains directly (${currentDomains.join(', ')}); move the composition into a task-domain service/workflow or add a narrow exception marker`
        )
      }
    }

    if (extension === WXML_EXTENSION) {
      const lines = content.split('\n')
      const legacyLineCounts = buildLineCountMap(legacyContent)

      lines.forEach((line, index) => {
        if (!EXPLANATORY_SURFACE_REGEX.test(line)) {
          return
        }

        if (hasNearbyMarker(lines, index, ALLOW_EXPLANATORY_SURFACE)) {
          return
        }

        if (consumeCount(legacyLineCounts, line.trim())) {
          return
        }

        fileFailures.push(
          `line ${index + 1} introduces an explanatory-card style surface; express the task through structure, state, labels, and action hierarchy first`
        )
      })
    }

    if (fileFailures.length > 0) {
      failures.push({ relativePath, fileFailures })
    }
  }

  if (failures.length > 0) {
    console.error('Frontend architecture boundary gate failed. New Mini Program page/component drift must not flatten API shape into UI structure.')
    console.error(`Allowed exception markers: ${ALLOW_PAGE_API_COMPOSITION} <reason>, ${ALLOW_EXPLANATORY_SURFACE} <reason>`)
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const reason of failure.fileFailures) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-frontend-architecture-boundary: validated ${targetFiles.length} changed Mini Program surface file(s)`)
}

main()