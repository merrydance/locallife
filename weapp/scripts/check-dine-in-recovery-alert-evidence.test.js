const assert = require('assert')
const fs = require('fs')
const os = require('os')
const path = require('path')

const requiredSections = [
  '## Alert Target',
  '## Metric Coverage',
  '## Rule Definition',
  '## Routing And Ownership',
  '## Verification Evidence',
  '## Result',
]

const weappRoot = path.join(__dirname, '..')

const requiredNeedles = [
  'Alert rule owner:',
  'Target environment:',
  'Rule config link or evidence id:',
  'Metric list_error:',
  'Metric close_failed:',
  'PromQL or equivalent expression:',
  'Threshold and window:',
  'Receiver or on-call route:',
  'Notification channel:',
  'Firing or dry-run evidence:',
  'Backend version or commit:',
  'Verdict:',
]

const requiredMetrics = [
  'dine_in_checkout_recovery_scans_total{result="list_error"}',
  'dine_in_checkout_recovery_sessions_total{result="close_failed"}',
]

const forbiddenPlaceholders = [
  '<fill',
  '<todo',
  '<required',
  'TBD',
  'TODO',
]

function readEvidence(filePath) {
  assert(filePath, 'usage: node scripts/check-dine-in-recovery-alert-evidence.test.js <evidence.md>')
  return fs.readFileSync(filePath, 'utf8')
}

function assertPackageWiring() {
  const pkg = JSON.parse(fs.readFileSync(path.join(weappRoot, 'package.json'), 'utf8'))
  assert(
    pkg.scripts['check:dine-in-recovery-alert-evidence'],
    'package.json must expose the dine-in recovery alert evidence check'
  )
  assert(
    pkg.scripts['quality:check'].includes('check:dine-in-recovery-alert-evidence'),
    'quality:check must run the dine-in recovery alert evidence check'
  )
}

function assertEvidence(content) {
  for (const section of requiredSections) {
    assert(content.includes(section), `missing required section: ${section}`)
  }
  for (const needle of requiredNeedles) {
    assert(content.includes(needle), `missing required evidence field: ${needle}`)
  }
  for (const metric of requiredMetrics) {
    assert(content.includes(metric), `missing required metric: ${metric}`)
  }
  for (const placeholder of forbiddenPlaceholders) {
    assert(!content.toLowerCase().includes(placeholder.toLowerCase()), `evidence still contains placeholder: ${placeholder}`)
  }
  assert(/Verdict:\s*(pass|fail)/i.test(content), 'Verdict must be pass or fail')
}

function runFixtureSelfCheck() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'dine-in-recovery-alert-evidence-'))
  const evidencePath = path.join(tmpDir, 'evidence.md')
  const evidence = `# Dine-In Checkout Recovery Alert Evidence

## Alert Target

Alert rule owner: platform-sre
Target environment: staging
Rule config link or evidence id: monitoring/rules/dine-in-checkout-recovery.yml#L1

## Metric Coverage

Metric list_error: dine_in_checkout_recovery_scans_total{result="list_error"}
Metric close_failed: dine_in_checkout_recovery_sessions_total{result="close_failed"}

## Rule Definition

PromQL or equivalent expression: increase(dine_in_checkout_recovery_scans_total{result="list_error"}[10m]) > 0 or increase(dine_in_checkout_recovery_sessions_total{result="close_failed"}[10m]) > 0
Threshold and window: any increase over 10 minutes pages the checkout recovery route

## Routing And Ownership

Receiver or on-call route: checkout-recovery-primary
Notification channel: PagerDuty checkout-recovery-primary

## Verification Evidence

Firing or dry-run evidence: alert dry-run AR-2026-06-15-001 evaluated both expressions
Backend version or commit: 994c10db

## Result

Verdict: pass
`
  fs.writeFileSync(evidencePath, evidence)
  assertEvidence(readEvidence(evidencePath))
  fs.rmSync(tmpDir, { recursive: true, force: true })
}

function main() {
  assertPackageWiring()

  const evidencePath = process.argv[2]
  if (!evidencePath) {
    runFixtureSelfCheck()
    console.log('check-dine-in-recovery-alert-evidence: evidence schema contract passed')
    return
  }
  const content = readEvidence(evidencePath)
  assertEvidence(content)
  const kind = content.includes('Status: template only') ? 'template schema' : 'evidence'
  console.log(`check-dine-in-recovery-alert-evidence: ${evidencePath} ${kind} is complete`)
}

main()
