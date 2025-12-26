#!/bin/bash
# LocalLife Systemd Service Installation Script

set -e

echo "Installing LocalLife systemd service..."

# Copy service file
sudo cp /home/sam/locallife/locallife/locallife.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable locallife

# Stop any existing manual processes
pkill -f './main' 2>/dev/null || true

# Start the service
sudo systemctl start locallife

# Check status
sleep 2
sudo systemctl status locallife --no-pager

echo ""
echo "✅ LocalLife service installed successfully!"
echo ""
echo "Commands you can use:"
echo "  sudo systemctl start locallife    # 启动服务"
echo "  sudo systemctl stop locallife     # 停止服务"  
echo "  sudo systemctl restart locallife  # 重启服务"
echo "  sudo systemctl status locallife   # 查看状态"
echo "  journalctl -u locallife -f        # 查看实时日志"
