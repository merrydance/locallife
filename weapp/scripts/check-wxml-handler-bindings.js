const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getGateScope,
  getScopedFiles,
  normalizeRelativePath
} = require('./gate-utils')

const PAGE_ROOT = 'weapp/miniprogram/pages/'
const BINDING_PATTERN = /\b(?:catch|bind|capture-bind|capture-catch):?[A-Za-z0-9_-]+\s*=\s*"([^"]+)"/g
const IDENTIFIER_PATTERN = /^[A-Za-z_$][\w$]*$/
const STRING_LITERAL_IDENTIFIER_PATTERN = /['"]([A-Za-z_$][\w$]*)['"]/g

const ALLOWLIST = {
  'weapp/miniprogram/pages/merchant/finance/settlement-account/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-status.ts',
    handlers: ['onNavHeight', 'onRetry', 'onPrimaryAction', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/operator/finance/settlement-account/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-status.ts',
    handlers: ['onNavHeight', 'onRetry', 'onPrimaryAction', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/platform/finance/settlement-account/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-status.ts',
    handlers: ['onNavHeight', 'onRetry', 'onPrimaryAction', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/rider/settlement-account/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-status.ts',
    handlers: ['onNavHeight', 'onRetry', 'onPrimaryAction', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/merchant/finance/settlement-account/submit/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-submit.ts',
    handlers: ['onNavHeight', 'onRetry', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/operator/finance/settlement-account/submit/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-submit.ts',
    handlers: ['onNavHeight', 'onRetry', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/platform/finance/settlement-account/submit/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-submit.ts',
    handlers: ['onNavHeight', 'onRetry', 'onWaitPrimary']
  },
  'weapp/miniprogram/pages/rider/settlement-account/submit/index.wxml': {
    source: 'weapp/miniprogram/behaviors/baofu-settlement-submit.ts',
    handlers: ['onNavHeight', 'onRetry', 'onWaitPrimary']
  }
}

function getLineNumber(source, index) {
  return source.slice(0, index).split('\n').length
}

function getLineText(source, lineNumber) {
  return source.split('\n')[lineNumber - 1] || ''
}

function stripComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, '')
}

function stripLeadingLineComments(source) {
  let result = source.trim()
  while (result.startsWith('//')) {
    const newlineIndex = result.indexOf('\n')
    if (newlineIndex < 0) {
      return ''
    }
    result = result.slice(newlineIndex + 1).trim()
  }
  return result
}

function isEscaped(source, index) {
  let slashCount = 0
  for (let cursor = index - 1; cursor >= 0 && source[cursor] === '\\'; cursor -= 1) {
    slashCount += 1
  }
  return slashCount % 2 === 1
}

function isLineCommentStart(source, index) {
  return source[index] === '/' && source[index + 1] === '/' && !isEscaped(source, index)
}

function getPageScriptPath(currentRepoRoot, wxmlRelativePath) {
  const parsed = path.parse(wxmlRelativePath)
  const tsPath = path.join(currentRepoRoot, parsed.dir, `${parsed.name}.ts`)
  if (fs.existsSync(tsPath)) {
    return tsPath
  }

  const jsPath = path.join(currentRepoRoot, parsed.dir, `${parsed.name}.js`)
  if (fs.existsSync(jsPath)) {
    return jsPath
  }

  return ''
}

function normalizeImportPath(fromScriptPath, importPath) {
  if (!importPath.startsWith('.')) {
    return ''
  }

  const importFilePath = path.resolve(path.dirname(fromScriptPath), importPath)
  const candidates = [
    importFilePath,
    `${importFilePath}.ts`,
    `${importFilePath}.js`,
    path.join(importFilePath, 'index.ts'),
    path.join(importFilePath, 'index.js')
  ]

  return candidates.find((candidate) => fs.existsSync(candidate)) || ''
}

function collectExportedObjectMethodNames(source, exportName) {
  const cleanSource = stripComments(source)
  const exportIndex = cleanSource.search(new RegExp(`\\b(?:export\\s+)?const\\s+${exportName}\\b`))
  if (exportIndex < 0) {
    return new Set()
  }

  const objectStart = cleanSource.indexOf('{', exportIndex)
  if (objectStart < 0) {
    return new Set()
  }

  const objectBody = readBalancedBlock(cleanSource, objectStart)
  return collectObjectMethodNames(objectBody)
}

function collectObjectMethodNames(objectBody) {
  const methodNames = new Set()
  const entries = splitTopLevelObjectEntries(objectBody)

  for (const entry of entries) {
    const trimmed = stripLeadingLineComments(entry)
    if (!trimmed || trimmed.startsWith('...')) {
      continue
    }

    const shorthandMatch = trimmed.match(/^(?:async\s+)?([A-Za-z_$][\w$]*)\s*\(/)
    if (shorthandMatch) {
      methodNames.add(shorthandMatch[1])
      continue
    }

    const propertyMatch = trimmed.match(/^([A-Za-z_$][\w$]*)\s*:/)
    if (propertyMatch) {
      methodNames.add(propertyMatch[1])
    }
  }

  return methodNames
}

function splitTopLevelObjectEntries(objectBody) {
  const entries = []
  let start = 0
  let braceDepth = 0
  let bracketDepth = 0
  let parenDepth = 0
  let quote = ''
  let escaped = false

  for (let index = 0; index < objectBody.length; index += 1) {
    const char = objectBody[index]

    if (quote) {
      if (escaped) {
        escaped = false
        continue
      }
      if (char === '\\') {
        escaped = true
        continue
      }
      if (char === quote) {
        quote = ''
      }
      continue
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char
      continue
    }

    if (isLineCommentStart(objectBody, index)) {
      const newlineIndex = objectBody.indexOf('\n', index + 2)
      if (newlineIndex < 0) {
        break
      }
      index = newlineIndex
      continue
    }

    if (char === '{') braceDepth += 1
    if (char === '}') braceDepth -= 1
    if (char === '[') bracketDepth += 1
    if (char === ']') bracketDepth -= 1
    if (char === '(') parenDepth += 1
    if (char === ')') parenDepth -= 1

    if (char === ',' && braceDepth === 0 && bracketDepth === 0 && parenDepth === 0) {
      entries.push(objectBody.slice(start, index))
      start = index + 1
    }
  }

  entries.push(objectBody.slice(start))
  return entries
}

function readBalancedBlock(source, startIndex) {
  let depth = 0
  let quote = ''
  let escaped = false

  for (let index = startIndex; index < source.length; index += 1) {
    const char = source[index]

    if (quote) {
      if (escaped) {
        escaped = false
        continue
      }
      if (char === '\\') {
        escaped = true
        continue
      }
      if (char === quote) {
        quote = ''
      }
      continue
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char
      continue
    }

    if (isLineCommentStart(source, index)) {
      const newlineIndex = source.indexOf('\n', index + 2)
      if (newlineIndex < 0) {
        break
      }
      index = newlineIndex
      continue
    }

    if (char === '{') {
      depth += 1
      continue
    }

    if (char === '}') {
      depth -= 1
      if (depth === 0) {
        return source.slice(startIndex + 1, index)
      }
    }
  }

  return ''
}

function collectScriptMethodNames(currentRepoRoot, scriptPath) {
  if (!scriptPath || !fs.existsSync(scriptPath)) {
    return new Set()
  }

  const source = fs.readFileSync(scriptPath, 'utf8')
  const cleanSource = stripComments(source)
  const methodNames = new Set()
  const pageMatch = /\bPage\s*\(/.exec(cleanSource)
  const pageIndex = pageMatch ? pageMatch.index : -1
  const pageObjectStart = pageIndex >= 0 ? cleanSource.indexOf('{', pageIndex) : -1

  if (pageObjectStart >= 0) {
    const pageObjectBody = readBalancedBlock(cleanSource, pageObjectStart)
    for (const methodName of collectObjectMethodNames(pageObjectBody)) {
      methodNames.add(methodName)
    }
  }

  const importSources = new Map()
  const importPattern = /import\s+\{([\s\S]*?)\}\s+from\s+['"]([^'"]+)['"]/g
  let importMatch = importPattern.exec(cleanSource)
  while (importMatch) {
    for (const rawName of importMatch[1].split(',')) {
      const importedName = rawName.trim().split(/\s+as\s+/)[1] || rawName.trim().split(/\s+as\s+/)[0]
      if (importedName) {
        importSources.set(importedName.trim(), importMatch[2])
      }
    }
    importMatch = importPattern.exec(cleanSource)
  }

  const spreadPattern = /\.\.\.\s*([A-Za-z_$][\w$]*)/g
  let spreadMatch = spreadPattern.exec(cleanSource)
  while (spreadMatch) {
    const spreadName = spreadMatch[1]
    const importSource = importSources.get(spreadName)
    if (!importSource) {
      spreadMatch = spreadPattern.exec(cleanSource)
      continue
    }

    const importedScriptPath = normalizeImportPath(scriptPath, importSource)
    if (!importedScriptPath) {
      spreadMatch = spreadPattern.exec(cleanSource)
      continue
    }

    const importedSource = fs.readFileSync(importedScriptPath, 'utf8')
    for (const methodName of collectExportedObjectMethodNames(importedSource, spreadName)) {
      methodNames.add(methodName)
    }
    spreadMatch = spreadPattern.exec(cleanSource)
  }

  return methodNames
}

function collectBoundHandlerNames(source) {
  const handlers = []
  let match = BINDING_PATTERN.exec(source)

  while (match) {
    const rawValue = match[1]
    const names = new Set()

    if (IDENTIFIER_PATTERN.test(rawValue)) {
      names.add(rawValue)
    } else {
      let stringMatch = STRING_LITERAL_IDENTIFIER_PATTERN.exec(rawValue)
      while (stringMatch) {
        names.add(stringMatch[1])
        stringMatch = STRING_LITERAL_IDENTIFIER_PATTERN.exec(rawValue)
      }
    }

    for (const handlerName of names) {
      handlers.push({
        handlerName,
        index: match.index,
        rawValue
      })
    }

    match = BINDING_PATTERN.exec(source)
  }

  return handlers
}

function isAllowed(relativePath, handlerName, allowlist) {
  const entry = allowlist[relativePath]
  return !!entry && entry.handlers.includes(handlerName)
}

function collectWxmlHandlerBindingFailures(options = {}) {
  const currentRepoRoot = options.repoRoot || repoRoot
  const wxmlFiles = options.wxmlFiles || getScopedFiles({ roots: [PAGE_ROOT], extensions: ['.wxml'] })
  const allowlist = options.allowlist || ALLOWLIST
  const failures = []

  for (const relativePath of wxmlFiles) {
    const normalizedPath = normalizeRelativePath(relativePath)
    const absolutePath = path.join(currentRepoRoot, normalizedPath)
    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const scriptPath = getPageScriptPath(currentRepoRoot, normalizedPath)
    if (!scriptPath) {
      continue
    }

    const scriptMethodNames = collectScriptMethodNames(currentRepoRoot, scriptPath)
    const source = fs.readFileSync(absolutePath, 'utf8')

    for (const binding of collectBoundHandlerNames(source)) {
      if (scriptMethodNames.has(binding.handlerName) || isAllowed(normalizedPath, binding.handlerName, allowlist)) {
        continue
      }

      const lineNumber = getLineNumber(source, binding.index)
      failures.push({
        relativePath: normalizedPath,
        lineNumber,
        handlerName: binding.handlerName,
        rawValue: binding.rawValue,
        lineText: getLineText(source, lineNumber).trim()
      })
    }
  }

  return failures
}

function main() {
  const changedWxmlFiles = getScopedFiles({ roots: [PAGE_ROOT], extensions: ['.wxml'] })

  if (changedWxmlFiles.length === 0) {
    console.log(`check-wxml-handler-bindings: no ${getGateScope() === 'changed' ? 'changed' : 'scannable'} page WXML files detected`)
    return
  }

  const failures = collectWxmlHandlerBindingFailures({ wxmlFiles: changedWxmlFiles })

  if (failures.length > 0) {
    console.error('WXML handler binding gate failed. Page WXML references handlers that are not visible on the page script.')
    console.error('Add the real handler, remove the stale binding, or add a documented allowlist entry with the behavior/runtime source.')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.relativePath}:${failure.lineNumber}`)
      console.error(`  - missing handler: ${failure.handlerName}`)
      console.error(`  - binding value: ${failure.rawValue}`)
      console.error(`  - ${failure.lineText}`)
    }

    process.exit(1)
  }

  console.log(`check-wxml-handler-bindings: validated ${changedWxmlFiles.length} page WXML file(s)`)
}

if (require.main === module) {
  main()
}

module.exports = {
  collectBoundHandlerNames,
  collectScriptMethodNames,
  collectWxmlHandlerBindingFailures
}
