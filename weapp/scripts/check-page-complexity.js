const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getGateScope,
  getScopedFiles,
  getDiffBase,
  readFileAtRevision
} = require('./gate-utils')

const PAGE_ROOT = 'weapp/miniprogram/pages/'
const FILE_LIMITS = {
  '.ts': 650,
  '.js': 650,
  '.wxml': 450,
  '.wxss': 400
}
const PAGE_TOTAL_LIMIT = 1200
const GROWTH_TOLERANCE = 40
const INTERNAL_PAGE_SEGMENT = /\/_(?:api|components|main_shared|services|utils)\//

function countLines(content) {
  if (!content) {
    return 0
  }

  return content.split('\n').length
}

function shouldCheckFile(filePath) {
  const ext = path.extname(filePath)
  return filePath.startsWith(PAGE_ROOT) &&
    !INTERNAL_PAGE_SEGMENT.test(filePath) &&
    Object.prototype.hasOwnProperty.call(FILE_LIMITS, ext)
}

function getCurrentLineCount(relativePath) {
  const absolutePath = path.join(repoRoot, relativePath)
  if (!fs.existsSync(absolutePath)) {
    return 0
  }

  return countLines(fs.readFileSync(absolutePath, 'utf8'))
}

function getPreviousLineCount(diffBase, relativePath) {
  return countLines(readFileAtRevision(diffBase, relativePath))
}

function main() {
  const scope = getGateScope()
  const diffBase = scope === 'changed' ? getDiffBase() : null
  const changedEntries = getScopedFiles({ roots: [PAGE_ROOT], extensions: Object.keys(FILE_LIMITS) })
    .filter((filePath) => shouldCheckFile(filePath))
    .map((filePath) => ({ filePath }))

  if (changedEntries.length === 0) {
    console.log(`check-page-complexity: no ${scope === 'changed' ? 'changed' : 'scannable'} Mini Program page source files detected`)
    return
  }

  const pageStats = new Map()

  for (const entry of changedEntries) {
    const pageDir = path.dirname(entry.filePath)
    const ext = path.extname(entry.filePath)
    const currentLines = getCurrentLineCount(entry.filePath)
    const previousLines = scope === 'changed' ? getPreviousLineCount(diffBase, entry.filePath) : 0

    if (!pageStats.has(pageDir)) {
      pageStats.set(pageDir, {
        files: [],
        currentTotal: 0,
        previousTotal: 0
      })
    }

    const stat = pageStats.get(pageDir)
    stat.files.push({
      relativePath: entry.filePath,
      ext,
      currentLines,
      previousLines,
      limit: FILE_LIMITS[ext]
    })
    stat.currentTotal += currentLines
    stat.previousTotal += previousLines
  }

  const failures = []

  for (const [pageDir, stat] of pageStats.entries()) {
    const reasons = []

    for (const file of stat.files) {
      const exceedsLimit = file.currentLines > file.limit
      const newlyExceeded = file.previousLines <= file.limit
      const grewTooMuchWhileAlreadyOver = file.previousLines > file.limit && file.currentLines > file.previousLines + GROWTH_TOLERANCE
      const shouldFail = scope === 'changed'
        ? exceedsLimit && (newlyExceeded || grewTooMuchWhileAlreadyOver)
        : exceedsLimit

      if (shouldFail) {
        reasons.push(
          `${path.basename(file.relativePath)} is ${file.currentLines} lines (limit ${file.limit}); split page or extract domain components instead of growing the page shell`
        )
      }
    }

    const exceedsTotalLimit = stat.currentTotal > PAGE_TOTAL_LIMIT
    const newlyExceededTotal = stat.previousTotal <= PAGE_TOTAL_LIMIT
    const grewTooMuchTotalWhileAlreadyOver = stat.previousTotal > PAGE_TOTAL_LIMIT && stat.currentTotal > stat.previousTotal + GROWTH_TOLERANCE
    const shouldFailTotal = scope === 'changed'
      ? exceedsTotalLimit && (newlyExceededTotal || grewTooMuchTotalWhileAlreadyOver)
      : exceedsTotalLimit

    if (shouldFailTotal) {
      reasons.push(
        `page total is ${stat.currentTotal} lines across changed page files (limit ${PAGE_TOTAL_LIMIT}); reassess task grouping, page boundary, or component extraction`
      )
    }

    if (reasons.length > 0) {
      failures.push({ pageDir, reasons })
    }
  }

  if (failures.length > 0) {
    console.error('Page complexity gate failed. Avoid turning one page into an oversized all-in-one surface.')
    if (scope === 'changed') {
      console.error(`A changed page fails when a source file newly crosses its limit or when an already-over-limit file grows by more than ${GROWTH_TOLERANCE} lines.`)
    } else {
      console.error('Full-scan mode fails every page that already exceeds the current page or file limit. Treat the result as a required refactor backlog, not a cosmetic warning.')
    }
    console.error(`Per-file limits: TS/JS ${FILE_LIMITS['.ts']} lines, WXML ${FILE_LIMITS['.wxml']} lines, WXSS ${FILE_LIMITS['.wxss']} lines. Page total limit: ${PAGE_TOTAL_LIMIT} lines.`)
    console.error('')

    for (const failure of failures) {
      console.error(failure.pageDir)
      for (const reason of failure.reasons) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-page-complexity: validated ${pageStats.size} page director${pageStats.size === 1 ? 'y' : 'ies'}`)
}

main()
