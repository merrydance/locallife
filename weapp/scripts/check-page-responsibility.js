const fs = require('fs')
const path = require('path')
const ts = require('typescript')
const { repoRoot, getGateScope, getScopedFiles } = require('./gate-utils')

const PAGE_ROOT = 'weapp/miniprogram/pages/'
const API_IMPORT_REGEX = /(?:from\s+['"](?:@\/|(?:\.\.\/)+)api\/([^/'"]+)|require\(\s*['"](?:@\/|(?:\.\.\/)+)api\/([^/'"]+))/g
const ALLOW_PAGE_DOMAIN_SPAN = 'weapp-gate allow-page-domain-span:'
const ALLOW_FIRST_SCREEN_FANOUT = 'weapp-gate allow-first-screen-fanout:'
const MAX_API_DOMAINS = 3
const MAX_AWAITS_PER_LIFECYCLE = 4
const MAX_PROMISE_ALL_ITEMS = 3
const LIFECYCLE_NAMES = new Set(['onLoad', 'onShow'])

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

function walk(node, visitor) {
  visitor(node)
  ts.forEachChild(node, (child) => walk(child, visitor))
}

function getObjectMethodBody(node) {
  if (ts.isMethodDeclaration(node) || ts.isFunctionExpression(node) || ts.isArrowFunction(node)) {
    return node.body || null
  }

  return null
}

function collectLifecycleMetrics(sourceFile) {
  const metrics = []

  walk(sourceFile, (node) => {
    if (!ts.isPropertyAssignment(node) && !ts.isMethodDeclaration(node)) {
      return
    }

    const nameNode = node.name
    if (!nameNode || !ts.isIdentifier(nameNode) || !LIFECYCLE_NAMES.has(nameNode.text)) {
      return
    }

    const body = ts.isMethodDeclaration(node)
      ? node.body
      : getObjectMethodBody(node.initializer)

    if (!body) {
      return
    }

    let awaitCount = 0
    const promiseAllItems = []

    walk(body, (innerNode) => {
      if (ts.isAwaitExpression(innerNode)) {
        awaitCount += 1
      }

      if (!ts.isCallExpression(innerNode) || !ts.isPropertyAccessExpression(innerNode.expression)) {
        return
      }

      if (innerNode.expression.expression.getText(sourceFile) !== 'Promise' || innerNode.expression.name.getText(sourceFile) !== 'all') {
        return
      }

      const [firstArg] = innerNode.arguments
      if (firstArg && ts.isArrayLiteralExpression(firstArg)) {
        promiseAllItems.push(firstArg.elements.length)
      }
    })

    metrics.push({
      lifecycleName: nameNode.text,
      awaitCount,
      promiseAllItems
    })
  })

  return metrics
}

function main() {
  const pageScripts = getScopedFiles({ roots: [PAGE_ROOT], extensions: ['.ts', '.js'] })

  if (pageScripts.length === 0) {
    console.log(`check-page-responsibility: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} Mini Program page scripts detected`)
    return
  }

  const failures = []

  for (const relativePath of pageScripts) {
    const absolutePath = path.join(repoRoot, relativePath)
    const content = fs.readFileSync(absolutePath, 'utf8')
    const fileFailures = []
    const domains = collectApiDomains(content)

    if (domains.length > MAX_API_DOMAINS && !content.includes(ALLOW_PAGE_DOMAIN_SPAN)) {
      fileFailures.push(
        `page spans ${domains.length} API domains (${domains.join(', ')}); extract a workflow-specific aggregator/service or split the page before exceeding one primary task`
      )
    }

    const sourceFile = ts.createSourceFile(absolutePath, content, ts.ScriptTarget.Latest, true)
    const lifecycleMetrics = collectLifecycleMetrics(sourceFile)

    for (const metric of lifecycleMetrics) {
      if (metric.awaitCount > MAX_AWAITS_PER_LIFECYCLE && !content.includes(ALLOW_FIRST_SCREEN_FANOUT)) {
        fileFailures.push(
          `${metric.lifecycleName} contains ${metric.awaitCount} await expressions; collapse first-screen loading behind a workflow aggregator or staged loading`
        )
      }

      const oversizedPromiseAll = metric.promiseAllItems.find((itemCount) => itemCount > MAX_PROMISE_ALL_ITEMS)
      if (oversizedPromiseAll && !content.includes(ALLOW_FIRST_SCREEN_FANOUT)) {
        fileFailures.push(
          `${metric.lifecycleName} uses Promise.all with ${oversizedPromiseAll} concurrent tasks; reduce first-screen fan-out or document the narrow exception`
        )
      }
    }

    if (fileFailures.length > 0) {
      failures.push({ relativePath, fileFailures })
    }
  }

  if (failures.length > 0) {
    console.error('Page responsibility gate failed. A page should keep one primary task, a bounded API-domain span, and restrained first-screen fan-out.')
    console.error(`Default thresholds: max ${MAX_API_DOMAINS} API domains per page, max ${MAX_AWAITS_PER_LIFECYCLE} awaits in onLoad/onShow, max Promise.all fan-out ${MAX_PROMISE_ALL_ITEMS}.`)
    console.error(`Use ${ALLOW_PAGE_DOMAIN_SPAN} or ${ALLOW_FIRST_SCREEN_FANOUT} only for narrow, documented exceptions.`)
    console.error('')

    for (const failure of failures) {
      console.error(failure.relativePath)
      for (const reason of failure.fileFailures) {
        console.error(`  - ${reason}`)
      }
    }

    process.exit(1)
  }

  console.log(`check-page-responsibility: validated ${pageScripts.length} page script file(s)`) 
}

main()