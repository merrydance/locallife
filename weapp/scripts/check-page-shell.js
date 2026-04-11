const path = require('path')
const {
  repoRoot,
  getChangedEntries,
  readFileIfExists,
  listFiles
} = require('./gate-utils')

const PAGE_ROOT = 'weapp/miniprogram/pages/'
const APPROVED_TOP_GAP = /padding-top:\s*calc\(\s*\{\{navBarHeight\}\}px\s*\+\s*var\(--spacer-sm\)\s*\)/
const APPROVED_PAGE_SHELL_TOP = /page-shell--with-nav/
const APPROVED_NAV_HEIGHT_VAR = /--page-shell-nav-height:\s*\{\{navBarHeight\}\}px/
const APPROVED_PAGE_SHELL_GUTTER = /page-shell--page-gutter/
const APPROVED_GUTTER = /var\(--spacer-md\)/
const APPROVED_PAGE_SHELL_SAFE = /page-shell--bottom-safe/
const APPROVED_SAFE_AREA = /var\(--safe-area-bottom\)|env\(safe-area-inset-bottom\)|--popup-bottom-padding-(?:sm|md|lg)/

function main() {
  const changedEntries = getChangedEntries()
  const pageDirs = Array.from(new Set(
    changedEntries
      .map((entry) => entry.filePath)
      .filter((filePath) => filePath.startsWith(PAGE_ROOT))
      .map((filePath) => path.dirname(filePath))
  ))

  if (pageDirs.length === 0) {
    console.log('check-page-shell: no changed Mini Program pages detected')
    return
  }

  const failures = []

  for (const pageDir of pageDirs) {
    const absoluteDir = path.join(repoRoot, pageDir)
    const pageFiles = [
      ...listFiles(absoluteDir, ['.wxml']),
      ...listFiles(absoluteDir, ['.wxss'])
    ]

    const combinedContent = pageFiles
      .map((filePath) => readFileIfExists(filePath))
      .join('\n')

    if (!combinedContent) {
      continue
    }

    const pageFailures = []
    const usesApprovedPageShell = APPROVED_PAGE_SHELL_TOP.test(combinedContent) && APPROVED_NAV_HEIGHT_VAR.test(combinedContent)
    const hasApprovedSafeArea = APPROVED_SAFE_AREA.test(combinedContent) || APPROVED_PAGE_SHELL_SAFE.test(combinedContent)

    if (!APPROVED_TOP_GAP.test(combinedContent) && !usesApprovedPageShell) {
      pageFailures.push('missing approved top-navigation gap pattern `calc({{navBarHeight}}px + var(--spacer-sm))`')
    }

    if (!APPROVED_GUTTER.test(combinedContent) && !APPROVED_PAGE_SHELL_GUTTER.test(combinedContent)) {
      pageFailures.push('missing approved horizontal gutter token `var(--spacer-md)`')
    }

    if (!hasApprovedSafeArea) {
      pageFailures.push('missing explicit bottom safe-area handling')
    }

    if (pageFailures.length > 0) {
      failures.push({
        pageDir,
        pageFailures
      })
    }
  }

  if (failures.length > 0) {
    console.error('Page shell gate failed. Changed pages must use the approved shell pattern:')
    console.error('- top gap: `padding-top: calc({{navBarHeight}}px + var(--spacer-sm))` or `.page-shell--with-nav` + `--page-shell-nav-height`')
    console.error('- page gutter: `var(--spacer-md)`')
    console.error('- bottom safe area: `.page-shell--bottom-safe`, `var(--safe-area-bottom)` or `env(safe-area-inset-bottom)`')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.pageDir}`)
      for (const reason of failure.pageFailures) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-page-shell: validated ${pageDirs.length} changed page directory(ies)`) 
}

main()