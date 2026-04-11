const fs = require('fs')
const path = require('path')
const {
  repoRoot,
  getChangedEntries
} = require('./gate-utils')

const WXML_ROOT = 'weapp/miniprogram/'
const UNSAFE_METHOD_PATTERNS = [
  {
    name: 'number formatting',
    regex: /\.\s*toFixed\s*\(/g
  },
  {
    name: 'string slicing',
    regex: /\.\s*(slice|substring|substr)\s*\(/g
  },
  {
    name: 'string replacement or trim',
    regex: /\.\s*(replace|replaceAll|trim|padStart|padEnd)\s*\(/g
  },
  {
    name: 'string case conversion',
    regex: /\.\s*(toUpperCase|toLowerCase)\s*\(/g
  },
  {
    name: 'array transformation',
    regex: /\.\s*(map|filter|reduce|join)\s*\(/g
  }
]

function getLineNumber(source, index) {
  return source.slice(0, index).split('\n').length
}

function getLineText(source, lineNumber) {
  return source.split('\n')[lineNumber - 1] || ''
}

function collectMustacheExpressions(source) {
  const expressions = []
  const regex = /\{\{([\s\S]*?)\}\}/g
  let match = regex.exec(source)

  while (match) {
    expressions.push({
      body: match[1],
      startIndex: match.index,
      raw: match[0]
    })
    match = regex.exec(source)
  }

  return expressions
}

function main() {
  const changedEntries = getChangedEntries()
  const changedWxmlFiles = Array.from(new Set(
    changedEntries
      .map((entry) => entry.filePath)
      .filter((filePath) => filePath.startsWith(WXML_ROOT) && filePath.endsWith('.wxml'))
  ))

  if (changedWxmlFiles.length === 0) {
    console.log('check-wxml-expression-safety: no changed WXML files detected')
    return
  }

  const failures = []

  for (const relativePath of changedWxmlFiles) {
    const absolutePath = path.join(repoRoot, relativePath)
    if (!fs.existsSync(absolutePath)) {
      continue
    }

    const content = fs.readFileSync(absolutePath, 'utf8')
    const expressions = collectMustacheExpressions(content)

    for (const expression of expressions) {
      for (const pattern of UNSAFE_METHOD_PATTERNS) {
        pattern.regex.lastIndex = 0
        const match = pattern.regex.exec(expression.body)
        if (!match) {
          continue
        }

        const lineNumber = getLineNumber(content, expression.startIndex)
        failures.push({
          relativePath,
          lineNumber,
          reason: `unsupported ${pattern.name} inside WXML expression`,
          lineText: getLineText(content, lineNumber).trim()
        })
      }
    }
  }

  if (failures.length > 0) {
    console.error('WXML expression safety gate failed. Do not use JS member-method formatting inside template expressions.')
    console.error('Move formatting into TS data fields or page methods before rendering.')
    console.error('')

    for (const failure of failures) {
      console.error(`${failure.relativePath}:${failure.lineNumber}`)
      console.error(`  - ${failure.reason}`)
      console.error(`  - ${failure.lineText}`)
    }

    process.exit(1)
  }

  console.log(`check-wxml-expression-safety: validated ${changedWxmlFiles.length} changed WXML file(s)`) 
}

main()