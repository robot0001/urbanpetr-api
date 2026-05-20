# Plan: RDS Tunnel via EC2 Instance Connect Endpoint

## Context

EC2 Instance Connect Endpoint (EICE) supports tunneling to any private VPC resource — not
just EC2 instances — using `aws ec2-instance-connect start-secure-tunnel`. The `open-tunnel`
sub-command (which we tried first) is restricted to ports 22/3389. `start-secure-tunnel`
has no such restriction and is the correct command for reaching RDS port 5432.

---

## What's Already in Place (No Terraform Changes Needed)

All infrastructure was deployed in a previous iteration:

| Resource | Name/ID | State |
|---|---|---|
| EIC endpoint | `urbanpetr-api-prod-eic-endpoint` | `create-complete` |
| EIC security group | `urbanpetr-api-prod-eic-endpoint` | Egress → RDS SG on 5432 |
| RDS security group | `urbanpetr-api-prod-lambda` (shared) | Ingress from EIC SG on 5432 |
| Secrets Manager secret | `urbanpetr/api/db/migrator` | Contains host/port/dbname/username/password |

---

## What Changes

### 1. `urbanpetr-api/scripts/db-tunnel.sh`

Replace the final `aws ec2-instance-connect open-tunnel` call with `start-secure-tunnel`.

**Current (broken — port 5432 rejected):**
```bash
aws ec2-instance-connect open-tunnel \
  --instance-connect-endpoint-id "$EIC_ID" \
  --private-ip-address "$RDS_IP" \
  --remote-port "$RDS_PORT" \
  --local-port "$LOCAL_PORT"
```

**Replacement:**
```bash
aws ec2-instance-connect start-secure-tunnel \
  --instance-connect-endpoint-id "$EIC_ID" \
  --instance-id "i-placeholder" \
  --target-ip-address "$RDS_IP" \
  --port "$RDS_PORT" \
  --local-port "$LOCAL_PORT"
```

Flag notes:
- `--instance-id i-placeholder` — required by CLI schema but ignored when targeting
  a non-EC2 resource; any `i-` prefixed string works.
- `--target-ip-address` replaces `--private-ip-address`.
- `--port` replaces `--remote-port`.
- `--instance-connect-endpoint-id` stays the same.

### 2. IAM — Personal Identity

Your personal IAM identity (used with `AWS_PROFILE=terraform`) needs the
`ec2-instance-connect:OpenTunnel` action on the EIC endpoint resource. This covers both
`open-tunnel` and `start-secure-tunnel` — they share the same IAM action.

**Policy to attach to your personal IAM user/role:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EICTunnel",
      "Effect": "Allow",
      "Action": "ec2-instance-connect:OpenTunnel",
      "Resource": "arn:aws:ec2:eu-central-1:<ACCOUNT_ID>:instance-connect-endpoint/*"
    },
    {
      "Sid": "EICDescribe",
      "Effect": "Allow",
      "Action": "ec2:DescribeInstanceConnectEndpoints",
      "Resource": "*"
    },
    {
      "Sid": "SecretsManagerMigrator",
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "arn:aws:secretsmanager:eu-central-1:<ACCOUNT_ID>:secret:urbanpetr/api/db/migrator-*"
    }
  ]
}
```

Replace `<ACCOUNT_ID>` with the prod Account A ID. This can be applied in the AWS console
under IAM → Users → your user → Add permissions → Attach policies → Create inline policy.

---

## Updated Script (Full)

```bash
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
#   aws CLI >= 2.13  (adds ec2-instance-connect start-secure-tunnel)
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

# start-secure-tunnel requires a private IP, not a hostname
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

aws ec2-instance-connect start-secure-tunnel \
  --instance-connect-endpoint-id "$EIC_ID" \
  --instance-id "i-placeholder" \
  --target-ip-address "$RDS_IP" \
  --port "$RDS_PORT" \
  --local-port "$LOCAL_PORT"
```

---

## Files Changed

| File | Change |
|---|---|
| `urbanpetr-api/scripts/db-tunnel.sh` | `open-tunnel` → `start-secure-tunnel` with updated flags |
| AWS Console (manual) | Inline policy on personal IAM identity |

No Terraform changes. No new resources. No CI/CD changes.

---

## Testing After Change

```bash
# 1. Run the tunnel
./scripts/db-tunnel.sh 5433

# 2. In a second terminal, connect
psql "host=localhost port=5433 dbname=<dbname> user=<user> password=<pass>"

# 3. Quick sanity check
SELECT current_database(), current_user, version();
```

Expected: tunnel prints the connect string and blocks; psql connects and returns one row.
