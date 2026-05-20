#!/usr/bin/env bash
# Opens a tunnel to the prod RDS instance via AWS SSM Session Manager.
# No bastion SSH keys, no open inbound ports — access is controlled by IAM.
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
#   aws CLI + session-manager-plugin  (brew install session-manager-plugin)
#   IAM permissions: ssm:StartSession on the bastion instance and document

set -euo pipefail

LOCAL_PORT="${1:-5433}"
SECRET_ID="urbanpetr/api/db/migrator"
BASTION_TAG="urbanpetr-prod-ssm-bastion"
export AWS_PROFILE="${AWS_PROFILE:-terraform}"

echo "Looking up SSM bastion ($BASTION_TAG)..."
BASTION_ID=$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=$BASTION_TAG" "Name=instance-state-name,Values=running" \
  --query 'Reservations[0].Instances[0].InstanceId' \
  --output text)

if [[ -z "$BASTION_ID" || "$BASTION_ID" == "None" ]]; then
  echo "Error: SSM bastion instance not found or not running." >&2
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

echo ""
echo "  Bastion:    $BASTION_ID"
echo "  RDS:        $RDS_HOST:$RDS_PORT"
echo "  Local port: $LOCAL_PORT"
echo ""
echo "Connect with:"
echo "  psql \"host=localhost port=$LOCAL_PORT dbname=$DB_NAME user=$DB_USER password=$DB_PASS\""
echo ""
echo "Tunnel open — press Ctrl-C to close."
echo ""

aws ssm start-session \
  --target "$BASTION_ID" \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters "{\"host\":[\"$RDS_HOST\"],\"portNumber\":[\"$RDS_PORT\"],\"localPortNumber\":[\"$LOCAL_PORT\"]}"
