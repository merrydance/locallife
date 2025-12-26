#!/bin/bash
TOKEN=$(psql -U sam -d locallife_dev -t -c "SELECT access_token FROM wechat_access_tokens WHERE app_type = 'mp';" | tr -d ' \n')
echo "Token prefix: ${TOKEN:0:30}..."
echo "Testing token validity..."
curl -s "https://api.weixin.qq.com/cgi-bin/getcallbackip?access_token=$TOKEN"
