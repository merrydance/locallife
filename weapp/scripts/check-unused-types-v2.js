/**
 * 检查未匹配 Swagger 的类型是否真正未使用
 * v2: 更准确的检查，考虑同文件内的使用
 */

const fs = require('fs');
const path = require('path');

// 读取 issues 文件获取未找到的类型
const issuesPath = path.join(__dirname, '../docs/api-swagger-diff-report.md');
const reportContent = fs.readFileSync(issuesPath, 'utf-8');

// 解析未找到的类型
const notFoundTypes = [];
const lines = reportContent.split('\n');
let currentFile = '';

for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const fileMatch = line.match(/^## (.+\.ts)$/);
    if (fileMatch) {
        currentFile = fileMatch[1];
        continue;
    }
    const typeMatch = line.match(/^### ❓ (.+)$/);
    if (typeMatch) {
        notFoundTypes.push({
            file: currentFile,
            typeName: typeMatch[1]
        });
    }
}

console.log(`找到 ${notFoundTypes.length} 个未匹配 Swagger 的类型\n`);

// 搜索目录（业务代码）
const searchDirs = [
    path.join(__dirname, '../miniprogram/pages'),
    path.join(__dirname, '../miniprogram/components'),
    path.join(__dirname, '../miniprogram/services'),
    path.join(__dirname, '../miniprogram/adapters'),
    path.join(__dirname, '../miniprogram/models'),
    path.join(__dirname, '../miniprogram/utils'),
];

function getAllTsFiles(dir) {
    const files = [];
    if (!fs.existsSync(dir)) return files;
    const items = fs.readdirSync(dir, { withFileTypes: true });
    for (const item of items) {
        const fullPath = path.join(dir, item.name);
        if (item.isDirectory()) {
            files.push(...getAllTsFiles(fullPath));
        } else if (item.name.endsWith('.ts') && !item.name.endsWith('.d.ts')) {
            files.push(fullPath);
        }
    }
    return files;
}

const allBusinessFiles = [];
for (const dir of searchDirs) {
    allBusinessFiles.push(...getAllTsFiles(dir));
}

const apiDir = path.join(__dirname, '../miniprogram/api');
const apiFiles = getAllTsFiles(apiDir);

console.log(`搜索 ${allBusinessFiles.length} 个业务文件和 ${apiFiles.length} 个 API 文件\n`);

// 检查类型是否在同文件内被使用（排除定义本身）
function isUsedInSameFile(filePath, typeName) {
    const content = fs.readFileSync(filePath, 'utf-8');

    // 移除类型定义本身，然后检查是否还有其他引用
    // 匹配 export interface TypeName 或 export type TypeName
    const defPattern = new RegExp(`export\\s+(interface|type)\\s+${typeName}\\b[^]*?(?=export\\s+(interface|type|class|function|const|async)|$)`, 'g');
    const contentWithoutDef = content.replace(defPattern, '');

    // 检查是否还有其他引用
    const usagePattern = new RegExp(`\\b${typeName}\\b`, 'g');
    const matches = contentWithoutDef.match(usagePattern);

    return matches && matches.length > 0;
}

// 检查类型是否被导出并在其他文件中使用
function isExportedAndUsed(apiFilePath, typeName, allFiles) {
    const apiContent = fs.readFileSync(apiFilePath, 'utf-8');

    // 检查是否被导出
    const isExported = new RegExp(`export\\s+(interface|type)\\s+${typeName}\\b`).test(apiContent);
    if (!isExported) return { exported: false, usedElsewhere: false };

    // 检查其他文件是否导入并使用
    for (const file of allFiles) {
        if (file === apiFilePath) continue;

        const content = fs.readFileSync(file, 'utf-8');

        // 检查是否从该API文件导入了这个类型
        const apiFileName = path.basename(apiFilePath, '.ts');
        const importPattern = new RegExp(`import\\s+.*\\b${typeName}\\b.*from\\s+['"]\\.\\.\\/api\\/${apiFileName}['"]`);

        if (importPattern.test(content)) {
            return { exported: true, usedElsewhere: true, usedIn: path.relative(path.join(__dirname, '..'), file) };
        }

        // 也检查直接使用（可能通过其他方式导入）
        if (new RegExp(`\\b${typeName}\\b`).test(content)) {
            // 确认不是同名的其他类型
            const hasImport = content.includes(`'../api/${apiFileName}'`) || content.includes(`"../api/${apiFileName}"`);
            if (hasImport) {
                return { exported: true, usedElsewhere: true, usedIn: path.relative(path.join(__dirname, '..'), file) };
            }
        }
    }

    return { exported: true, usedElsewhere: false };
}

const results = {
    usedInBusiness: [],      // 在业务代码中使用
    usedInSameFile: [],      // 在同文件内被使用（如作为函数参数）
    usedInOtherApi: [],      // 在其他API文件中使用
    trulyUnused: []          // 真正未使用
};

for (const { file, typeName } of notFoundTypes) {
    const apiFilePath = path.join(__dirname, '../miniprogram/api', file);

    // 1. 检查是否在业务代码中使用
    let usedInBusiness = false;
    let businessLocation = '';
    for (const businessFile of allBusinessFiles) {
        const content = fs.readFileSync(businessFile, 'utf-8');
        if (new RegExp(`\\b${typeName}\\b`).test(content)) {
            usedInBusiness = true;
            businessLocation = path.relative(path.join(__dirname, '..'), businessFile);
            break;
        }
    }

    if (usedInBusiness) {
        results.usedInBusiness.push({ file, typeName, usedIn: businessLocation });
        continue;
    }

    // 2. 检查是否在同文件内被使用
    if (fs.existsSync(apiFilePath) && isUsedInSameFile(apiFilePath, typeName)) {
        results.usedInSameFile.push({ file, typeName });
        continue;
    }

    // 3. 检查是否在其他API文件中使用
    let usedInOtherApi = false;
    let otherApiLocation = '';
    for (const otherApiFile of apiFiles) {
        if (path.basename(otherApiFile) === file) continue;
        const content = fs.readFileSync(otherApiFile, 'utf-8');
        if (new RegExp(`\\b${typeName}\\b`).test(content)) {
            usedInOtherApi = true;
            otherApiLocation = path.relative(path.join(__dirname, '..'), otherApiFile);
            break;
        }
    }

    if (usedInOtherApi) {
        results.usedInOtherApi.push({ file, typeName, usedIn: otherApiLocation });
        continue;
    }

    // 4. 真正未使用
    results.trulyUnused.push({ file, typeName });
}

// 输出结果
console.log('='.repeat(60));
console.log(`✅ 在业务代码中使用: ${results.usedInBusiness.length} 个`);
console.log('='.repeat(60));
results.usedInBusiness.forEach(item => console.log(`  ${item.file} -> ${item.typeName} (${item.usedIn})`));

console.log('\n' + '='.repeat(60));
console.log(`📦 在同文件内使用（如函数参数）: ${results.usedInSameFile.length} 个`);
console.log('='.repeat(60));
results.usedInSameFile.forEach(item => console.log(`  ${item.file} -> ${item.typeName}`));

console.log('\n' + '='.repeat(60));
console.log(`🔗 在其他API文件中使用: ${results.usedInOtherApi.length} 个`);
console.log('='.repeat(60));
results.usedInOtherApi.forEach(item => console.log(`  ${item.file} -> ${item.typeName} (${item.usedIn})`));

console.log('\n' + '='.repeat(60));
console.log(`❌ 真正未使用（可删除）: ${results.trulyUnused.length} 个`);
console.log('='.repeat(60));

// 按文件分组显示
const byFile = {};
results.trulyUnused.forEach(item => {
    if (!byFile[item.file]) byFile[item.file] = [];
    byFile[item.file].push(item.typeName);
});

Object.entries(byFile).forEach(([file, types]) => {
    console.log(`\n  ${file}:`);
    types.forEach(t => console.log(`    - ${t}`));
});

// 保存报告
const report = {
    summary: {
        total: notFoundTypes.length,
        usedInBusiness: results.usedInBusiness.length,
        usedInSameFile: results.usedInSameFile.length,
        usedInOtherApi: results.usedInOtherApi.length,
        trulyUnused: results.trulyUnused.length
    },
    usedInBusiness: results.usedInBusiness,
    usedInSameFile: results.usedInSameFile,
    usedInOtherApi: results.usedInOtherApi,
    trulyUnused: results.trulyUnused,
    trulyUnusedByFile: byFile
};

fs.writeFileSync(
    path.join(__dirname, '../docs/unused-types-report-v2.json'),
    JSON.stringify(report, null, 2)
);

console.log('\n\n报告已保存到: docs/unused-types-report-v2.json');
