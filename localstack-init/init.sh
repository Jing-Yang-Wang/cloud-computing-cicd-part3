#!/bin/bash
set -e

echo "Initializing DynamoDB and S3..."

# Create DynamoDB table if it does not exist.
if ! awslocal dynamodb describe-table \
    --table-name Items >/dev/null 2>&1; then

    awslocal dynamodb create-table \
        --table-name Items \
        --attribute-definitions AttributeName=id,AttributeType=N \
        --key-schema AttributeName=id,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST
fi

# Create S3 bucket if it does not exist.
if ! awslocal s3api head-bucket \
    --bucket items-bucket >/dev/null 2>&1; then

    awslocal s3api create-bucket \
        --bucket items-bucket
fi

echo "DynamoDB table and S3 bucket are ready!"