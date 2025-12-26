/**
 * 安全的批量替换脚本 - 逐文件处理并验证
 * 使用方法: node scripts/safe-replace-console.js
 */

const fs = require('fs');
const path = require('path');

// 需要处理的文件列表
const filesToProcess = [
    'miniprogram/pages/user_center/addresses/edit/index.ts',
    'miniprogram/pages/takeout/order-confirm/index.ts',
    'miniprogram/pages/rider/tasks/index.ts',
    'miniprogram/pages/rider/task-detail/index.ts',
    'miniprogram/pages/rider/deposit/index.ts',
    'miniprogram/pages/rider/dashboard/index.ts',
    'miniprogram/pages/reservation/index.ts',
    'miniprogram/pages/register/rider/index.ts',
    'miniprogram/pages/register/operator/index.ts',
    'miniprogram/pages/register/merchant/index.ts',
    'miniprogram/pages/orders/list/index.ts',
    'miniprogram/pages/orders/detail/index.ts',
    'miniprogram/pages/merchant/orders/index.ts',
    'miniprogram/pages/merchant/dishes/index.ts',
    'miniprogram/pages/merchant/dishes/edit/index.ts',
    'miniprogram/pages/merchant/dashboard/index.ts',
    'miniprogram/pages/merchant/analytics/enhanced/index.ts',
    'miniprogram/pages/dining/index.ts'
];

// 安全替换函数 - 只替换完整的console调用
function safeReplaceConsole(content, filePath) {
    let modified = false;
    const lines = content.split('\n');
    const newLines = [];

    for (let i = 0; i < lines.length; i++) {
        let line = lines[i];
        const originalLine = line;

        // 1. 替换 console.error('message', error)
        if (line.includes('console.error(')) {
            const match = line.match(/console\.error\(['"](.+?)['"],\s*(.+?)\)/);
            if (match) {
                const indent = line.match(/^(\s*)/)[1];
                const message = match[1];
                const errorVar = match[2];
                line = `${indent}logger.error('${message}', ${errorVar}, '${getContext(filePath)}')`;
                modified = true;
            }
        }

        // 2. 替换 console.warn('message')
        else if (line.includes('console.warn(')) {
            const match = line.match(/console\.warn\(['"](.+?)['"]\)/);
            if (match) {
                const indent = line.match(/^(\s*)/)[1];
                const message = match[1];
                line = `${indent}logger.warn('${message}', undefined, '${getContext(filePath)}')`;
                modified = true;
            }
        }

        // 3. 替换 console.log('message', data)
        else if (line.includes('console.log(')) {
            const match = line.match(/console\.log\(['"](.+?)['"],\s*(.+?)\)/);
            if (match) {
                const indent = line.match(/^(\s*)/)[1];
                const message = match[1];
                const data = match[2];
                line = `${indent}logger.debug('${message}', ${data}, '${getContext(filePath)}')`;
                modified = true;
            }
        }

        // 4. 删除注释掉的console
        else if (line.trim().startsWith('// console.')) {
            continue; // 跳过这一行
        }

        newLines.push(line);
    }

    return { content: newLines.join('\n'), modified };
}

function getContext(filePath) {
    const parts = filePath.split('/');
    const pageName = parts[parts.length - 2];
    return pageName.charAt(0).toUpperCase() + pageName.slice(1);
}

function addImports(content) {
    // 检查是否已有import
    if (content.includes("import { logger }") && content.includes("import { ErrorHandler }")) {
        return content;
    }

    const lines = content.split('\n');
    let firstImportIndex = -1;

    // 找到第一个import语句
    for (let i = 0; i < lines.length; i++) {
        if (lines[i].trim().startsWith('import ')) {
            firstImportIndex = i;
            break;
        }
    }

    if (firstImportIndex === -1) {
        // 没有import,在文件开头添加
        const imports = [
            "import { logger } from '../../../utils/logger'",
            "import { ErrorHandler } from '../../../utils/error-handler'",
            ""
        ];
        return imports.join('\n') + content;
    }

    // 在第一个import之后添加
    const imports = [];
    if (!content.includes("import { logger }")) {
        imports.push("import { logger } from '../../../utils/logger'");
    }
    if (!content.includes("import { ErrorHandler }")) {
        imports.push("import { ErrorHandler } from '../../../utils/error-handler'");
    }

    if (imports.length > 0) {
        lines.splice(firstImportIndex + 1, 0, ...imports);
    }

    return lines.join('\n');
}

function processFile(filePath) {
    const fullPath = path.join(__dirname, '..', filePath);

    if (!fs.existsSync(fullPath)) {
        console.log(`❌ 文件不存在: ${filePath}`);
        return false;
    }

    try {
        let content = fs.readFileSync(fullPath, 'utf8');
        const original = content;

        // 1. 替换console调用
        const { content: replacedContent, modified } = safeReplaceConsole(content, filePath);

        if (!modified) {
            console.log(`⏭️  无需更新: ${filePath}`);
            return true;
        }

        // 2. 添加imports
        content = addImports(replacedContent);

        // 3. 验证语法 - 检查括号匹配
        if (!validateSyntax(content)) {
            console.log(`⚠️  语法验证失败,跳过: ${filePath}`);
            return false;
        }

        // 4. 创建备份
        fs.writeFileSync(fullPath + '.backup', original, 'utf8');

        // 5. 写入新内容
        fs.writeFileSync(fullPath, content, 'utf8');

        console.log(`✅ 已更新: ${filePath}`);
        return true;
    } catch (error) {
        console.error(`❌ 处理失败: ${filePath}`, error.message);
        return false;
    }
}

function validateSyntax(content) {
    // 简单的括号匹配检查
    const stack = [];
    const pairs = { '(': ')', '[': ']', '{': '}', "'": "'", '"': '"' };
    let inString = false;
    let stringChar = '';

    for (let i = 0; i < content.length; i++) {
        const char = content[i];
        const prevChar = i > 0 ? content[i - 1] : '';

        // 处理字符串
        if ((char === "'" || char === '"') && prevChar !== '\\') {
            if (!inString) {
                inString = true;
                stringChar = char;
                stack.push(char);
            } else if (char === stringChar) {
                inString = false;
                stringChar = '';
                stack.pop();
            }
        }

        // 不在字符串中时检查括号
        if (!inString) {
            if ('([{'.includes(char)) {
                stack.push(char);
            } else if (')]}'.includes(char)) {
                const last = stack[stack.length - 1];
                if (last && pairs[last] === char) {
                    stack.pop();
                } else {
                    return false; // 括号不匹配
                }
            }
        }
    }

    return stack.length === 0; // 所有括号都应该匹配
}

// 主执行
console.log('🚀 开始安全批量替换...\n');
let successCount = 0;
let failCount = 0;

filesToProcess.forEach(file => {
    if (processFile(file)) {
        successCount++;
    } else {
        failCount++;
    }
});

console.log(`\n📊 处理完成:`);
console.log(`   ✅ 成功: ${successCount}个文件`);
console.log(`   ❌ 失败: ${failCount}个文件`);
console.log(`\n💡 提示: 备份文件保存为 .backup 后缀,如有问题可恢复`);
