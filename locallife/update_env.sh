#!/bin/bash
# Update domain
sed -i 's/locallifeapi.merrydance.cn/llapi.merrydance.cn/g' app.env
# Clear redis password for local dev
sed -i 's/REDIS_PASSWORD=".*/REDIS_PASSWORD=""/g' app.env
echo "app.env updated."
