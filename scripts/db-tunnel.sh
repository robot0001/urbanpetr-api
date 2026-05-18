#!/usr/bin/env bash
# Opens an SSH tunnel to the prod RDS instance via a bastion host.
# Credentials are pulled from Secrets Manager; the tunnel runs until you Ctrl-C.
#
# Usage:
#   ./scripts/db-tunnel.sh [local_port] [bastion_host]
#
# Defaults:
#   local_port   5433
#   bastion_host value of $BASTION env var, or prompt
#
# Then connect with:
#   psql "host=localhost port=<local_port> dbname=<dbname> user=<user> password=<pass>"

set -euo pipefail

LOCAL_PORT="${1:-5433}"
BASTION="${2:-${BASTION:-}}"
SECRET_ID="urbanpetr/api/db/migrator"
SSH_USER="${SSH_USER:-ec2-user}"
SSH_KEY_ARGS=()
if [[ -n "${SSH_KEY:-}" ]]; then
  SSH_KEY_ARGS=(-i "$SSH_KEY")
fi

if [[ -z "$BASTION" ]]; then
  read -rp "Bastion host: " BASTION
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
echo "  RDS:        $RDS_HOST:$RDS_PORT"
echo "  Bastion:    $SSH_USER@$BASTION"
echo "  Local port: $LOCAL_PORT"
echo ""
echo "Connect with:"
echo "  psql \"host=localhost port=$LOCAL_PORT dbname=$DB_NAME user=$DB_USER password=$DB_PASS\""
echo ""
echo "Tunnel open — press Ctrl-C to close."
echo ""

ssh -N \
  -L "${LOCAL_PORT}:${RDS_HOST}:${RDS_PORT}" \
  "${SSH_KEY_ARGS[@]}" \
  "${SSH_USER}@${BASTION}"
