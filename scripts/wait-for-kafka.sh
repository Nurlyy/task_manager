#!/bin/sh
set -e

KAFKA_HOST="kafka"
KAFKA_PORT="9092"
MAX_RETRIES=30
RETRY_INTERVAL=2

echo "Waiting for Kafka to be available at $KAFKA_HOST:$KAFKA_PORT..."
retry_count=0

while [ $retry_count -lt $MAX_RETRIES ]; do
  if nc -z $KAFKA_HOST $KAFKA_PORT; then
    echo "Kafka is available!"
    break
  fi
  
  retry_count=$((retry_count+1))
  echo "Attempt $retry_count/$MAX_RETRIES - Kafka not available yet, retrying in ${RETRY_INTERVAL}s..."
  sleep $RETRY_INTERVAL
done

if [ $retry_count -eq $MAX_RETRIES ]; then
  echo "Kafka is still not available after $MAX_RETRIES attempts - proceeding anyway"
fi

# Execute the passed command
exec "$@"