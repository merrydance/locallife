const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries,
  getGateScope,
  getScopedFiles,
  normalizeRelativePath
} = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const ALLOW_IMPORT_MARKER = 'weapp-gate allow-super-service-import:'

const PROTECTED_SUPER_SERVICES = [
  {
    servicePath: 'weapp/miniprogram/services/operator-console.ts',
    importPattern: /from\s+['"][^'"]*services\/operator-console['"]|require\(\s*['"][^'"]*services\/operator-console['"]\s*\)/,
    name: 'operator-console',
    allowedImporters: new Set([]),
    ownershipNotePath: 'weapp/docs/architecture-ownership/operator-console.md'
  },
  {
    servicePath: 'weapp/miniprogram/services/merchant-console.ts',
    importPattern: /from\s+['"][^'"]*services\/merchant-console['"]|require\(\s*['"][^'"]*services\/merchant-console['"]\s*\)/,
    name: 'merchant-console',
    allowedImporters: new Set([]),
    ownershipNotePath: 'weapp/docs/architecture-ownership/merchant-console.md'
  }
]

function main() {
  const changedFiles = new Set(getChangedEntries().map((entry) => normalizeRelativePath(entry.filePath)))
  const scopedFiles = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.ts', '.js'] })

  if (scopedFiles.length === 0) {
    console.log(`check-super-service-boundary: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} Mini Program page/component scripts detected`)
    return
  }

  const failures = []

  for (const relativePath of scopedFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const fileFailures = []

    for (const protectedService of PROTECTED_SUPER_SERVICES) {
      if (!protectedService.importPattern.test(content)) {
        continue
      }

      if (protectedService.allowedImporters.has(relativePath)) {
        continue
      }

      if (content.includes(`${ALLOW_IMPORT_MARKER} ${protectedService.name}`)) {
        continue
      }

      fileFailures.push(
        `pages/components must not import protected super service ${protectedService.name}; extract a task-domain service or add a narrow, reviewed exception marker`
      )
    }

    if (fileFailures.length > 0) {
      failures.push({ relativePath, fileFailures })
    }
  }

  for (const protectedService of PROTECTED_SUPER_SERVICES) {
    if (!changedFiles.has(protectedService.servicePath)) {
      continue
    }

    if (changedFiles.has(protectedService.ownershipNotePath)) {
      continue
    }

    failures.push({
      relativePath: protectedService.servicePath,
      fileFailures: [
        `protected super service changed without ownership note update: ${protectedService.ownershipNotePath}`
      ]
    })
  }

  if (failures.length > 0) {
    console.error('Super service boundary gate failed. Protected super services cannot silently absorb new page/component dependencies or behavior changes.')
    console.error(`Allowed exception marker: ${ALLOW_IMPORT_MARKER} <service-name>`) 
    console.error('When a protected super service changes, update its ownership note in the same change set.')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const reason of failure.fileFailures) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-super-service-boundary: validated ${scopedFiles.length} script file(s) and ${PROTECTED_SUPER_SERVICES.length} protected super service definition(s)`)
}

main()