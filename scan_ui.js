const fs = require('fs');
const path = require('path');

function walk(dir) {
    let results = [];
    const list = fs.readdirSync(dir);
    list.forEach(function(file) {
        file = dir + '/' + file;
        const stat = fs.statSync(file);
        if (stat && stat.isDirectory()) { 
            results = results.concat(walk(file));
        } else { 
            if (file.endsWith('.wxml')) results.push(file);
        }
    });
    return results;
}

const files = walk('./weapp/miniprogram');
let nativeButtons = [];
let missingRoundButtons = [];
let darkTags = [];
let primaryRetryButtons = [];

files.forEach(file => {
    const content = fs.readFileSync(file, 'utf8');
    const lines = content.split('\n');
    lines.forEach((line, index) => {
        const lineNum = index + 1;
        // Native buttons without open-type
        if (line.match(/<button[^>]*>/) && !line.includes('open-type=') && !line.includes('button-hover')) {
             nativeButtons.push(`${file}:${lineNum}: ${line.trim()}`);
        }
        // t-buttons missing shape="round"
        if (line.match(/<t-button[^>]*>/) && !line.includes('shape="round"') && !line.includes('shape=\'round\'') && !line.includes('shape="circle"')) {
             missingRoundButtons.push(`${file}:${lineNum}: ${line.trim()}`);
        }
        // t-tags with variant="dark" 
        if (line.match(/<t-tag[^>]*>/) && line.includes('variant="dark"')) {
             darkTags.push(`${file}:${lineNum}: ${line.trim()}`);
        }
        // secondary actions with theme="primary" but not outline
        if (line.match(/<t-button[^>]*>/) && line.includes('theme="primary"') && !line.includes('variant="outline"') && !line.includes('variant="text"')) {
             if (line.includes('取消') || line.includes('重试') || line.includes('返回')) {
                 primaryRetryButtons.push(`${file}:${lineNum}: ${line.trim()}`);
             }
        }
    });
});

console.log('--- Native Buttons (Non open-type) ---');
console.log(nativeButtons.join('\n'));
console.log('\n--- Missing shape="round" on <t-button> ---');
console.log(missingRoundButtons.join('\n'));
console.log('\n--- <t-tag> using variant="dark" ---');
console.log(darkTags.join('\n'));
console.log('\n--- Secondary buttons (Retry/Cancel) using theme="primary" without outline ---');
console.log(primaryRetryButtons.join('\n'));
