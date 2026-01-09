/**
 * API 与 Swagger 定义校验脚本
 * 自动对比 TypeScript interface 与 Swagger definitions，生成差异报告
 * 
 * 用法: node scripts/validate-api-swagger.js
 */

const fs = require('fs')
const path = require('path')

// 配置
const SWAGGER_PATH = path.join(__dirname, '../docs/swagger.json')
const API_DIR = path.join(__dirname, '../miniprogram/api')
const OUTPUT_PATH = path.join(__dirname, '../docs/api-swagger-diff-report.md')

// 加载 Swagger
function loadSwagger() {
    const content = fs.readFileSync(SWAGGER_PATH, 'utf-8')
    return JSON.parse(content)
}

// 从 Swagger 提取所有 definitions
function extractSwaggerDefinitions(swagger) {
    const definitions = {}

    if (swagger.definitions) {
        for (const [name, def] of Object.entries(swagger.definitions)) {
            if (def.properties) {
                definitions[name] = {
                    properties: Object.keys(def.properties).sort(),
                    details: def.properties
                }
            }
        }
    }

    return definitions
}

// 从 TypeScript 文件提取 interface 定义
function extractTsInterfaces(filePath) {
    const content = fs.readFileSync(filePath, 'utf-8')
    const interfaces = {}

    // 使用更精确的方式匹配 interface，处理嵌套大括号
    const lines = content.split('\n')
    let currentInterface = null
    let braceCount = 0
    let bodyLines = []

    for (const line of lines) {
        // 检测 interface 开始
        const interfaceMatch = line.match(/export\s+interface\s+(\w+)\s*(?:extends\s+[\w,\s<>]+)?\s*\{/)
        if (interfaceMatch && braceCount === 0) {
            currentInterface = interfaceMatch[1]
            braceCount = 1
            bodyLines = []
            continue
        }

        if (currentInterface) {
            // 计算大括号
            for (const char of line) {
                if (char === '{') braceCount++
                else if (char === '}') braceCount--
            }

            if (braceCount > 0) {
                bodyLines.push(line)
            } else {
                // interface 结束，只提取顶层属性（braceCount 为 1 时的属性）
                const properties = []
                let depth = 0
                for (const bodyLine of bodyLines) {
                    // 计算当前行之前的深度
                    const openBraces = (bodyLine.match(/\{/g) || []).length
                    const closeBraces = (bodyLine.match(/\}/g) || []).length

                    // 只在顶层（depth === 0）提取属性
                    if (depth === 0) {
                        const propMatch = bodyLine.match(/^\s*(\w+)\??:/)
                        if (propMatch) {
                            properties.push(propMatch[1])
                        }
                    }

                    depth += openBraces - closeBraces
                }

                interfaces[currentInterface] = {
                    properties: properties.sort(),
                    raw: bodyLines.join('\n')
                }
                currentInterface = null
            }
        }
    }

    return interfaces
}

// 将 TypeScript interface 名转换为可能的 Swagger 名称
function guessSwaggerName(tsName) {
    const candidates = []

    // 直接匹配
    candidates.push(`api.${tsName}`)
    candidates.push(`api.${tsName.charAt(0).toLowerCase() + tsName.slice(1)}`)

    // Response -> response
    if (tsName.endsWith('Response')) {
        const base = tsName.slice(0, -8)
        candidates.push(`api.${base.charAt(0).toLowerCase() + base.slice(1)}Response`)
        candidates.push(`api.${base.charAt(0).toLowerCase() + base.slice(1)}Res`)
    }

    // DTO 后缀
    if (tsName.endsWith('DTO')) {
        const base = tsName.slice(0, -3)
        candidates.push(`api.${base.charAt(0).toLowerCase() + base.slice(1)}Response`)
    }

    return candidates
}

// 对比两个属性列表
function compareProperties(tsProps, swaggerProps) {
    const missing = swaggerProps.filter(p => !tsProps.includes(p) && !tsProps.includes(camelToSnake(p)))
    const extra = tsProps.filter(p => !swaggerProps.includes(p) && !swaggerProps.includes(snakeToCamel(p)))
    const matched = tsProps.filter(p => swaggerProps.includes(p) || swaggerProps.includes(snakeToCamel(p)))

    return { missing, extra, matched }
}

// snake_case 转 camelCase
function snakeToCamel(str) {
    return str.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase())
}

// camelCase 转 snake_case
function camelToSnake(str) {
    return str.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`)
}

// 主函数
function main() {
    console.log('加载 Swagger 文档...')
    const swagger = loadSwagger()
    const swaggerDefs = extractSwaggerDefinitions(swagger)

    console.log(`找到 ${Object.keys(swaggerDefs).length} 个 Swagger definitions`)

    // 获取所有 API 文件
    const apiFiles = fs.readdirSync(API_DIR)
        .filter(f => f.endsWith('.ts') && !f.endsWith('.d.ts'))

    console.log(`找到 ${apiFiles.length} 个 API 文件`)

    const report = []
    const summary = {
        totalInterfaces: 0,
        matched: 0,
        mismatched: 0,
        notFound: 0,
        issues: []
    }

    report.push('# API 与 Swagger 定义校验报告')
    report.push('')
    report.push(`生成时间: ${new Date().toISOString()}`)
    report.push('')
    report.push('---')
    report.push('')

    for (const file of apiFiles) {
        const filePath = path.join(API_DIR, file)
        const interfaces = extractTsInterfaces(filePath)

        if (Object.keys(interfaces).length === 0) continue

        report.push(`## ${file}`)
        report.push('')

        for (const [tsName, tsInterface] of Object.entries(interfaces)) {
            summary.totalInterfaces++

            // 尝试找到对应的 Swagger 定义
            const candidates = guessSwaggerName(tsName)
            let swaggerDef = null
            let swaggerName = null

            for (const candidate of candidates) {
                if (swaggerDefs[candidate]) {
                    swaggerDef = swaggerDefs[candidate]
                    swaggerName = candidate
                    break
                }
            }

            // 注意：移除了模糊匹配逻辑，因为它会导致错误的匹配
            // 例如 FavoriteDishResponse 被错误匹配到 api.dishResponse
            // 只使用精确匹配，未找到的类型标记为"未找到"

            if (!swaggerDef) {
                summary.notFound++
                report.push(`### ❓ ${tsName}`)
                report.push('')
                report.push('> 未找到对应的 Swagger 定义')
                report.push('')
                report.push(`TS 属性: ${tsInterface.properties.join(', ')}`)
                report.push('')
                continue
            }

            // 对比属性
            const comparison = compareProperties(tsInterface.properties, swaggerDef.properties)

            if (comparison.missing.length === 0 && comparison.extra.length === 0) {
                summary.matched++
                report.push(`### ✅ ${tsName} → ${swaggerName}`)
                report.push('')
                report.push('完全匹配')
                report.push('')
            } else {
                summary.mismatched++
                report.push(`### ❌ ${tsName} → ${swaggerName}`)
                report.push('')

                if (comparison.missing.length > 0) {
                    report.push(`**缺少字段 (Swagger 有但 TS 没有):**`)
                    for (const prop of comparison.missing) {
                        const detail = swaggerDef.details[prop]
                        const type = detail?.type || detail?.$ref?.split('/').pop() || 'unknown'
                        report.push(`- \`${prop}\`: ${type}`)
                        summary.issues.push({
                            file,
                            interface: tsName,
                            type: 'missing',
                            field: prop,
                            swaggerType: type
                        })
                    }
                    report.push('')
                }

                if (comparison.extra.length > 0) {
                    report.push(`**多余字段 (TS 有但 Swagger 没有):**`)
                    for (const prop of comparison.extra) {
                        report.push(`- \`${prop}\``)
                        summary.issues.push({
                            file,
                            interface: tsName,
                            type: 'extra',
                            field: prop
                        })
                    }
                    report.push('')
                }
            }
        }
    }

    // 添加摘要
    const summarySection = [
        '# 摘要',
        '',
        `- 总 Interface 数: ${summary.totalInterfaces}`,
        `- ✅ 完全匹配: ${summary.matched}`,
        `- ❌ 存在差异: ${summary.mismatched}`,
        `- ❓ 未找到对应: ${summary.notFound}`,
        `- 总问题数: ${summary.issues.length}`,
        '',
        '---',
        ''
    ]

    report.splice(4, 0, ...summarySection)

    // 写入报告
    fs.writeFileSync(OUTPUT_PATH, report.join('\n'))
    console.log(`\n报告已生成: ${OUTPUT_PATH}`)
    console.log(`\n摘要:`)
    console.log(`  总 Interface: ${summary.totalInterfaces}`)
    console.log(`  ✅ 匹配: ${summary.matched}`)
    console.log(`  ❌ 差异: ${summary.mismatched}`)
    console.log(`  ❓ 未找到: ${summary.notFound}`)

    // 输出问题 JSON 供后续处理
    const issuesPath = path.join(__dirname, '../docs/api-swagger-issues.json')
    fs.writeFileSync(issuesPath, JSON.stringify(summary.issues, null, 2))
    console.log(`\n问题列表已保存: ${issuesPath}`)
}

main()
