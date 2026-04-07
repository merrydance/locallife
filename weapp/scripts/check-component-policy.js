const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries,
  getDiffBase,
  pathExistsInRevision
} = require('./gate-utils')

const COMPONENT_ROOT = 'weapp/miniprogram/components/'
const POLICY_FILE = 'component-policy.json'
const ALLOWED_GROUPS = new Set(['feedback', 'data', 'navigation', 'base', 'form'])
const ALLOWED_DECISIONS = new Set(['tdesign-wrapper', 'local-component'])

function main() {
  const diffBase = getDiffBase()
  const changedEntries = getChangedEntries()

  const componentDirs = Array.from(new Set(
    changedEntries
      .map((entry) => entry.filePath)
      .filter((filePath) => filePath.startsWith(COMPONENT_ROOT))
      .map((filePath) => filePath.slice(COMPONENT_ROOT.length).split('/')[0])
      .filter(Boolean)
  ))

  const newComponentDirs = componentDirs.filter((dirName) => {
    const relativePath = `${COMPONENT_ROOT}${dirName}`
    return !pathExistsInRevision(diffBase, relativePath)
  })

  if (newComponentDirs.length === 0) {
    console.log('check-component-policy: no new shared Mini Program components detected')
    return
  }

  const failures = []

  for (const dirName of newComponentDirs) {
    const policyPath = path.join(repoRoot, COMPONENT_ROOT, dirName, POLICY_FILE)

    if (!fs.existsSync(policyPath)) {
      failures.push({
        dirName,
        reason: `missing ${POLICY_FILE}`
      })
      continue
    }

    let policy
    try {
      policy = JSON.parse(fs.readFileSync(policyPath, 'utf8'))
    } catch (error) {
      failures.push({
        dirName,
        reason: `${POLICY_FILE} is not valid JSON`
      })
      continue
    }

    const reasons = []

    if (typeof policy.purpose !== 'string' || policy.purpose.trim().length === 0) {
      reasons.push('`purpose` must be a non-empty string')
    }

    if (!ALLOWED_GROUPS.has(policy.tdesignGroup)) {
      reasons.push('`tdesignGroup` must be one of: feedback, data, navigation, base, form')
    }

    if (!Array.isArray(policy.tdesignCandidates) || policy.tdesignCandidates.length === 0) {
      reasons.push('`tdesignCandidates` must be a non-empty array')
    }

    if (!ALLOWED_DECISIONS.has(policy.decision)) {
      reasons.push('`decision` must be `tdesign-wrapper` or `local-component`')
    }

    if (typeof policy.rationale !== 'string' || policy.rationale.trim().length === 0) {
      reasons.push('`rationale` must explain why direct TDesign usage was insufficient')
    }

    if (reasons.length > 0) {
      failures.push({
        dirName,
        reason: reasons.join('; ')
      })
    }
  }

  if (failures.length > 0) {
    console.error('Component policy gate failed. New shared components must declare why TDesign was not enough.')
    console.error('Required file: component-policy.json')
    console.error('Required fields: purpose, tdesignGroup, tdesignCandidates, decision, rationale')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.dirName}`)
      console.error(`  - ${failure.reason}`)
    }

    process.exit(1)
  }

  console.log(`check-component-policy: validated ${newComponentDirs.length} new component directory(ies)`) 
}

main()