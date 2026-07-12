#!/bin/sh
set -e

# Install AWS CLI if not present (needed for ECR authentication)
if ! command -v aws &> /dev/null; then
    echo "Installing AWS CLI..."
    apt-get update -qq && \
    apt-get install -y -qq awscli > /dev/null && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
fi

# Install Docker CLI if not present (needed for docker login command)
if ! command -v docker &> /dev/null; then
    echo "Installing Docker CLI..."
    apt-get update -qq && \
    apt-get install -y -qq docker.io > /dev/null && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
fi

# Verify tools are available
aws --version
docker --version

# Execute the command passed as arguments
exec "$@"

