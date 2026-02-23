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
let modifiedFiles = 0;

files.forEach(file => {
    let content = fs.readFileSync(file, 'utf8');
    let original = content;

    // 1. native button in payment-success
    if (file.includes('payment-success.wxml')) {
        content = content.replace(/<button class="action-btn primary" bindtap="goToOrderDetail">\s*查看订单详情\s*<\/button>/, '<t-button theme="primary" shape="round" block size="large" bindtap="goToOrderDetail" style="margin-bottom: 32rpx;">\n      查看订单详情\n    </t-button>');
        content = content.replace(/<button class="action-btn secondary" bindtap="goToHome">\s*返回首页\s*<\/button>/, '<t-button theme="default" variant="outline" shape="round" block size="large" bindtap="goToHome">\n      返回首页\n    </t-button>');
    }

    // 2. Add shape="round" to <t-button> if missing
    content = content.replace(/<t-button([^>]*?)>/g, (match, p1) => {
        if (!p1.includes('shape="round"') && !p1.includes("shape='round'") && !p1.includes('shape="circle"')) {
            return `<t-button${p1} shape="round">`;
        }
        return match;
    });

    // 3. Downgrade heavy retry/cancel buttons
    content = content.replace(/<t-button([^>]*?)>([^<]*?)<\/t-button>/g, (match, p1, p2) => {
        if (p1.includes('theme="primary"') && !p1.includes('variant="outline"') && !p1.includes('variant="text"')) {
            if (p2.includes('重试') || p2.includes('返回') || p2.includes('取消')) {
                // Add variant="outline"
                return `<t-button${p1} variant="outline">${p2}</t-button>`;
            }
        }
        return match;
    });

    // 4. Transform variant="dark" to variant="light-outline" for tags, except promo tags
    if (!file.includes('pages/takeout/merchant-info/index.wxml')) {
        content = content.replace(/<t-tag([^>]*?)variant="dark"([^>]*?)>/g, '<t-tag$1variant="light-outline"$2>');
    } else {
        // Only replace line 31
        content = content.replace(/<t-tag wx:for="\{\{restaurant\.tags\}\}"([^>]*?)variant="dark"([^>]*?)>/g, '<t-tag wx:for="{{restaurant.tags}}"$1variant="light-outline"$2>');
    }

    if (content !== original) {
        fs.writeFileSync(file, content, 'utf8');
        modifiedFiles++;
    }
});

console.log('Modified ' + modifiedFiles + ' files.');
