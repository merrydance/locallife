const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const { repoRoot, getGateScope, getScopedFiles } = require('./gate-utils')

const SURFACE_ROOTS = ['weapp/miniprogram/pages/', 'weapp/miniprogram/components/']
const ALLOWLIST = new Set([
  'weapp/miniprogram/pages/user_center/index.ts',
  'weapp/miniprogram/pages/register/operator/index.ts'
])
const ROLE_BOUNDARY_SEGMENT = /\/_(?:api|main_shared\/api|main_shared\/services|main_shared\/utils|services|utils)\//
const FORBIDDEN_CONSOLE_ROLES = new Set(['admin', 'operator', 'merchant', 'rider', 'customer', 'guest'])

function shouldCheckFile(filePath) {
  return !ALLOWLIST.has(filePath) && !ROLE_BOUNDARY_SEGMENT.test(filePath)
}

function main() {
  const changedFiles = getScopedFiles({ roots: SURFACE_ROOTS, extensions: ['.ts', '.js'] })
    .filter(shouldCheckFile)

  if (changedFiles.length === 0) {
    console.log(`check-role-contract: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} Mini Program page/component scripts detected`)
    return
  }

  const failures = []

  for (const relativePath of changedFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const sourceFile = ts.createSourceFile(absolutePath, content, ts.ScriptTarget.Latest, true)
    const fileFailures = []

    walk(sourceFile, (node) => {
      if (ts.isBinaryExpression(node) && isRoleComparison(node)) {
        fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not hardcode console role decisions; go through console-access or shared role helpers'))
      }

      if (ts.isCallExpression(node) && isRoleMembershipCheck(node)) {
        fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not infer console access from raw role arrays; go through console-access or shared role helpers'))
      }

      if (ts.isCaseClause(node) && isRoleCase(node.parent, node)) {
        fileFailures.push(formatFailure(node, sourceFile, 'pages/components must not switch on hardcoded console roles outside the shared role boundary'))
      }
    })

    if (fileFailures.length > 0) {
      failures.push({
        relativePath,
        fileFailures
      })
    }
  }

  if (failures.length > 0) {
    console.error('Role contract gate failed. Console role routing must stay in shared role helpers, not page/component code.')
    console.error('Allowed central files: pages/user_center/index.ts, pages/register/operator/index.ts')
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const line of failure.fileFailures) {
        console.error(`  - ${line}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-role-contract: validated ${changedFiles.length} script file(s)`) 
}

function walk(node, visitor) {
  visitor(node)
  ts.forEachChild(node, (child) => walk(child, visitor))
}

function formatFailure(node, sourceFile, message) {
  const position = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile))
  return `L${position.line + 1}: ${message}`
}

function isRoleComparison(node) {
  const operator = node.operatorToken.kind
  const isEqualityOperator = operator === ts.SyntaxKind.EqualsEqualsEqualsToken
    || operator === ts.SyntaxKind.EqualsEqualsToken
    || operator === ts.SyntaxKind.ExclamationEqualsEqualsToken
    || operator === ts.SyntaxKind.ExclamationEqualsToken

  if (!isEqualityOperator) {
    return false
  }

  const leftLiteral = getForbiddenRoleLiteral(node.left)
  const rightLiteral = getForbiddenRoleLiteral(node.right)
  const subject = leftLiteral ? node.right : rightLiteral ? node.left : null

  return !!subject && looksLikeRoleSubject(subject)
}

function isRoleMembershipCheck(node) {
  if (!ts.isPropertyAccessExpression(node.expression)) {
    return false
  }

  const methodName = node.expression.name.getText()
  if (methodName !== 'includes' || node.arguments.length !== 1) {
    return false
  }

  return !!getForbiddenRoleLiteral(node.arguments[0]) && looksLikeRoleSubject(node.expression.expression)
}

function isRoleCase(parent, node) {
  if (!parent || !ts.isCaseBlock(parent)) {
    return false
  }

  const switchStatement = parent.parent
  if (!switchStatement || !ts.isSwitchStatement(switchStatement)) {
    return false
  }

  return !!getForbiddenRoleLiteral(node.expression) && looksLikeRoleSubject(switchStatement.expression)
}

function getForbiddenRoleLiteral(node) {
  if (!ts.isStringLiteralLike(node)) {
    return null
  }

  return FORBIDDEN_CONSOLE_ROLES.has(node.text) ? node.text : null
}

function looksLikeRoleSubject(node) {
  const text = node.getText().toLowerCase()
  return [
    'role',
    'roles',
    'permission',
    'permissions',
    'access',
    'scope',
    'console',
    'workbench'
  ].some((token) => text.includes(token))
}

main()
