/**
 * 批量替换console调用的脚本
 * 使用方法: 在项目根目录运行 node scripts/replace-console.js
 */

const fs = require('fs');
const path = require('path');

// 需要处理的文件列表
const filesToProcess = [
    'miniprogram/pages/user_center/addresses/index.ts',
    'miniprogram/pages/user_center/addresses/edit/index.ts',
    'miniprogram/pages/takeout/order-confirm/index.ts',
    'miniprogram/pages/takeout/index.ts',
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

function replaceConsoleInFile(filePath) {
    const fullPath = path.join(__dirname, '..', filePath);

    if (!fs.existsSync(fullPath)) {
        console.log(`文件不存在: ${filePath}`);
        return;
    }

    let content = fs.readFileSync(fullPath, 'utf8');
    let modified = false;

    // 检查是否已经导入logger
    const hasLoggerImport = content.includes("import { logger } from");
    const hasErrorHandlerImport = content.includes("import { ErrorHandler } from");

    // 添加导入语句
    if (!hasLoggerImport || !hasErrorHandlerImport) {
        const imports = [];
        if (!hasLoggerImport) {
            imports.push("import { logger } from '../../utils/logger'");
        }
        if (!hasErrorHandlerImport) {
            imports.push("import { ErrorHandler } from '../../utils/error-handler'");
        }

        // 在第一个import之后添加
        const firstImportMatch = content.match(/^import .+$/m);
        if (firstImportMatch) {
            const insertPos = firstImportMatch.index + firstImportMatch[0].length;
            content = content.slice(0, insertPos) + '\n' + imports.join('\n') + content.slice(insertPos);
            modified = true;
        }
    }

    // 替换console.error
    const errorPattern = /console\.error\(['"](.+?)['"],\s*(.+?)\)/g;
    content = content.replace(errorPattern, (match, message, errorVar) => {
        modified = true;
        return `logger.error('${message}', ${errorVar}, '${getContextFromFile(filePath)}')`;
    });

    // 替换console.warn
    const warnPattern = /console\.warn\(['"](.+?)['"],?\s*(.+?)?\)/g;
    content = content.replace(warnPattern, (match, message, data) => {
        modified = true;
        const dataArg = data ? `, ${data}` : ', undefined';
        return `logger.warn('${message}'${dataArg}, '${getContextFromFile(filePath)}')`;
    });

    // 替换console.log
    const logPattern = /console\.log\(['"](.+?)['"],?\s*(.+?)?\)/g;
    content = content.replace(logPattern, (match, message, data) => {
        modified = true;
        const dataArg = data ? `, ${data}` : ', undefined';
        return `logger.debug('${message}'${dataArg}, '${getContextFromFile(filePath)}')`;
    });

    // 注释掉的console调用也要处理
    content = content.replace(/\/\/\s*console\.(error|warn|log)\(.+?\)/g, '');

    if (modified) {
        fs.writeFileSync(fullPath, content, 'utf8');
        console.log(`✅ 已更新: ${filePath}`);
    } else {
        console.log(`⏭️  无需更新: ${filePath}`);
    }
}

function getContextFromFile(filePath) {
    const parts = filePath.split('/');
    const fileName = parts[parts.length - 2];
    return fileName.charAt(0).toUpperCase() + fileName.slice(1);
}

// 执行批量替换
console.log('开始批量替换console调用...\n');
filesToProcess.forEach(replaceConsoleInFile);
console.log('\n✨ 批量替换完成!');
