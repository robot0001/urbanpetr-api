#!/usr/bin/env bash
# Opens a tunnel to the prod RDS instance via the EC2 Instance Connect Endpoint.
# No bastion, no SSH keys — access is controlled by your IAM identity.
#
# Usage:
#   ./scripts/db-tunnel.sh [local_port]
#
# Default local_port: 5433
#
# Then connect with:
#   psql "host=localhost port=<local_port> dbname=<dbname> user=<user> password=<pass>"
#
# Requirements:
#   aws CLI >= 2.13  (adds ec2-instance-connect open-tunnel)
#   IAM permission: ec2-instance-connect:OpenTunnel on the EIC endpoint

set -euo pipefail

LOCAL_PORT="${1:-5433}"
SECRET_ID="urbanpetr/api/db/migrator"
EIC_TAG="urbanpetr-api-prod-eic-endpoint"
export AWS_PROFILE="${AWS_PROFILE:-terraform}"

echo "Looking up EIC endpoint ($EIC_TAG)..."
EIC_ID=$(aws ec2 describe-instance-connect-endpoints \
  --filters "Name=tag:Name,Values=$EIC_TAG" "Name=state,Values=create-complete" \
  --query 'InstanceConnectEndpoints[0].InstanceConnectEndpointId' \
  --output text)

if [[ -z "$EIC_ID" || "$EIC_ID" == "None" ]]; then
  echo "Error: EIC endpoint not found or not in create-complete state." >&2
  exit 1
fi

echo "Fetching credentials from Secrets Manager ($SECRET_ID)..."
SECRET=$(aws secretsmanager get-secret-value \
  --secret-id "$SECRET_ID" \
  --query SecretString \
  --output text)

RDS_HOST=$(echo "$SECRET" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['host'])")
RDS_PORT=$(echo "$SECRET" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('port', 5432))")
DB_NAME=$(echo "$SECRET"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['dbname'])")
DB_USER=$(echo "$SECRET"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['username'])")
DB_PASS=$(echo "$SECRET"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['password'])")

# EIC open-tunnel requires a private IP, not a hostname
RDS_IP=$(python3 -c "import socket; print(socket.gethostbyname('$RDS_HOST'))")

echo ""
echo "  EIC endpoint: $EIC_ID"
echo "  RDS:          $RDS_HOST ($RDS_IP):$RDS_PORT"
echo "  Local port:   $LOCAL_PORT"
echo ""
echo "Connect with:"
echo "  psql \"host=localhost port=$LOCAL_PORT dbname=$DB_NAME user=$DB_USER password=$DB_PASS\""
echo ""
echo "Tunnel open — press Ctrl-C to close."
echo ""

aws ec2-instance-connect open-tunnel \
  --instance-connect-endpoint-id "$EIC_ID" \
  --private-ip-address "$RDS_IP" \
  --remote-port "$RDS_PORT" \
  --local-port "$LOCAL_PORT"
