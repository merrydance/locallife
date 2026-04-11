const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries
} = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const DIRECT_REQUEST = /\bwx\.request\s*\(/
const REQUEST_IMPORT = /from\s+['"][^'"]*utils\/request['"]|require\(\s*['"][^'"]*utils\/request['"]\s*\)/

function main() {
  const changedFiles = getChangedEntries()
    .map((entry) => entry.filePath)
    .filter((filePath) => SURFACE_ROOTS.some((root) => filePath.startsWith(root)))
    .filter((filePath) => ['.ts', '.js'].includes(path.extname(filePath)))

  if (changedFiles.length === 0) {
    console.log('check-request-boundary: no changed Mini Program page/component scripts detected')
    return
  }

  const failures = []

  for (const relativePath of changedFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const reasons = []

    if (DIRECT_REQUEST.test(content)) {
      reasons.push('direct `wx.request` usage is forbidden in pages/components')
    }

    if (REQUEST_IMPORT.test(content)) {
      reasons.push('pages/components must not import `utils/request` directly; go through api/ or services/')
    }

    if (reasons.length > 0) {
      failures.push({
        relativePath,
        reasons
      })
    }
  }

  if (failures.length > 0) {
    console.error('Request boundary gate failed. Pages/components must use the API or service layer, not raw request utilities.')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const reason of failure.reasons) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-request-boundary: validated ${changedFiles.length} changed script file(s)`) 
}

main()