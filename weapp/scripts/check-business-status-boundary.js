const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const { repoRoot, getGateScope, getScopedFiles } = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const ALLOWLIST = new Set([
  'weapp/miniprogram/pages/user_center/index.ts',
  'weapp/miniprogram/pages/register/operator/index.ts'
])
const BUSINESS_STATUS_LITERALS = new Set([
  'active',
  'approved',
  'rejected',
  'pending',
  'submitted',
  'reviewing',
  'pending_approval',
  'suspended',
  'offline',
  'draft',
  'bindbank_submitted',
  'auditing',
  'checking',
  'account_need_verify',
  'to_be_signed',
  'signing',
  'need_sign',
  'finish',
  'frozen',
  'rejected_sign',
  'canceled',
  'resolved',
  'paid',
  'refunded',
  'closed',
  'failed',
  'partial',
  'mixed',
  'unknown',
  'available',
  'occupied',
  'reserved',
  'disabled',
  'preparing',
  'ready',
  'courier_accepted',
  'picked',
  'delivering',
  'rider_delivered',
  'user_delivered',
  'completed',
  'cancelled'
])

function main() {
  const changedFiles = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.ts', '.js', '.wxml'] })
    .filter((filePath) => !ALLOWLIST.has(filePath))

  if (changedFiles.length === 0) {
    console.log(`check-business-status-boundary: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} Mini Program page/component files detected`)
    return
  }

  const failures = []

  for (const relativePath of changedFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const fileFailures = []

    if (path.extname(relativePath) === '.wxml') {
      fileFailures.push(...getTemplateStatusFailures(content))
    } else {
      const sourceFile = ts.createSourceFile(absolutePath, content, ts.ScriptTarget.Latest, true)

      walk(sourceFile, (node) => {
        if (ts.isBinaryExpression(node) && isBusinessStatusComparison(node)) {
          fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not compare business or review status strings directly; use shared status helpers'))
        }

        if (ts.isCallExpression(node) && isBusinessStatusIncludes(node)) {
          fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not hardcode business or review status arrays; use shared status helpers'))
        }

        if (ts.isCaseClause(node) && isBusinessStatusCase(node.parent, node)) {
          fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not switch on business or review status literals outside shared helpers'))
        }
      })
    }

    if (fileFailures.length > 0) {
      failures.push({ relativePath, fileFailures })
    }
  }

  if (failures.length > 0) {
    console.error('Status boundary gate failed. Pages/components must consume shared status helpers instead of hardcoded business or review status strings.')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const line of failure.fileFailures) {
        console.error(`  - ${line}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-business-status-boundary: validated ${changedFiles.length} file(s)`) 
}

function walk(node, visitor) {
  visitor(node)
  ts.forEachChild(node, (child) => walk(child, visitor))
}

function formatFailure(node, sourceFile, message) {
  const position = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile))
  return `L${position.line + 1}: ${message}`
}

function isBusinessStatusComparison(node) {
  const operator = node.operatorToken.kind
  const isEqualityOperator = operator === ts.SyntaxKind.EqualsEqualsEqualsToken
    || operator === ts.SyntaxKind.EqualsEqualsToken
    || operator === ts.SyntaxKind.ExclamationEqualsEqualsToken
    || operator === ts.SyntaxKind.ExclamationEqualsToken

  if (!isEqualityOperator) {
    return false
  }

  const leftLiteral = getBusinessStatusLiteral(node.left)
  const rightLiteral = getBusinessStatusLiteral(node.right)
  const subject = leftLiteral ? node.right : rightLiteral ? node.left : null

  return !!subject && looksLikeBusinessStatusSubject(subject)
}

function isBusinessStatusIncludes(node) {
  if (!ts.isPropertyAccessExpression(node.expression)) {
    return false
  }

  const methodName = node.expression.name.getText()
  if (methodName !== 'includes' || node.arguments.length !== 1) {
    return false
  }

  const subject = node.arguments[0]
  if (!looksLikeBusinessStatusSubject(subject)) {
    return false
  }

  const owner = node.expression.expression
  if (!ts.isArrayLiteralExpression(owner)) {
    return false
  }

  return owner.elements.some((element) => !!getBusinessStatusLiteral(element))
}

function isBusinessStatusCase(parent, node) {
  if (!parent || !ts.isCaseBlock(parent)) {
    return false
  }

  const switchStatement = parent.parent
  if (!switchStatement || !ts.isSwitchStatement(switchStatement)) {
    return false
  }

  return !!getBusinessStatusLiteral(node.expression) && looksLikeBusinessStatusSubject(switchStatement.expression)
}

function getBusinessStatusLiteral(node) {
  if (!ts.isStringLiteralLike(node)) {
    return null
  }

  return BUSINESS_STATUS_LITERALS.has(node.text) ? node.text : null
}

function looksLikeBusinessStatusSubject(node) {
  const text = node.getText().toLowerCase()

  if ([
    'ocr',
    'uploadfeedback',
    'licensestatus',
    'idfrontstatus',
    'idbackstatus',
    'ocrdisplaystate'
  ].some((token) => text.includes(token))) {
    return false
  }

  return [
    'status',
    'state',
    'payment',
    'order',
    'fulfillment',
    'delivery',
    'combined'
  ].some((token) => text.includes(token))
}

function getTemplateStatusFailures(content) {
  const failures = []
  const lines = content.split(/\r?\n/)
  const literalAlternation = Array.from(BUSINESS_STATUS_LITERALS).join('|')
  const subjectPattern = String.raw`(?:\.status\b|\.state\b|\bstatus\b(?![A-Za-z])|\borderStatus\b|\bpaymentStatus\b|\bfulfillmentStatus\b|\bdeliveryStatus\b|\bcombinedPaymentStatus\b)`
  const comparisonPattern = new RegExp(
    `${subjectPattern}[^\\n}]*[!=]==?\\s*['\"](${literalAlternation})['\"]`,
    'i'
  )

  lines.forEach((line, index) => {
    if (comparisonPattern.test(line)) {
      failures.push(`L${index + 1}: templates must not compare business or review status literals directly; use precomputed view fields from shared helpers`)
    }
  })

  return failures
}

main()