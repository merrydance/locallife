#!/bin/bash
set -e

# Create role 'sam' if not exists
sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='sam'" | grep -q 1 || \
sudo -u postgres psql -c "CREATE ROLE sam WITH LOGIN PASSWORD '123.' SUPERUSER;"

# Create database 'locallife_dev' if not exists
sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw locallife_dev || \
sudo -u postgres psql -c "CREATE DATABASE locallife_dev OWNER sam;"

echo "PostgreSQL user and database setup complete."
