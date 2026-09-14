import json
import os

import boto3

LAMBDA_NAMES = os.environ["LAMBDA_NAMES"].split(",")
WAF_IP_SET_ID = os.environ["WAF_IP_SET_ID"]
WAF_IP_SET_NAME = os.environ["WAF_IP_SET_NAME"]
WAF_IP_SET_V6_ID = os.environ["WAF_IP_SET_V6_ID"]
WAF_IP_SET_V6_NAME = os.environ["WAF_IP_SET_V6_NAME"]
CF_DIST_IDS = os.environ["CF_DIST_IDS"].split(",")


def handler(event, context):
    print(f"Kill switch triggered. Event: {json.dumps(event)}")

    # Every target is attempted even if an earlier one fails — a missing
    # function or a distribution mid-deploy must not leave the rest running.
    failures = []
    for name in LAMBDA_NAMES:
        _attempt(failures, f"Lambda {name}", _throttle_lambda, name)
    _attempt(failures, "WAF", _block_all_waf)
    for dist_id in CF_DIST_IDS:
        _attempt(failures, f"CloudFront {dist_id}", _disable_cloudfront, dist_id)

    if failures:
        # Raising makes SNS retry the async invoke; every step is idempotent.
        raise RuntimeError(f"Kill switch incomplete, failed: {failures}")

    return {"status": "killed"}


def _attempt(failures, label, fn, *args):
    try:
        fn(*args)
    except Exception as e:
        print(f"{label}: FAILED — {e}")
        failures.append(label)


def _throttle_lambda(name):
    boto3.client("lambda", region_name="eu-central-1").put_function_concurrency(
        FunctionName=name,
        ReservedConcurrentExecutions=0,
    )
    print(f"Lambda {name}: concurrency set to 0")


def _block_all_waf():
    waf = boto3.client("wafv2", region_name="us-east-1")

    r4 = waf.get_ip_set(Name=WAF_IP_SET_NAME, Scope="CLOUDFRONT", Id=WAF_IP_SET_ID)
    waf.update_ip_set(
        Name=WAF_IP_SET_NAME,
        Scope="CLOUDFRONT",
        Id=WAF_IP_SET_ID,
        LockToken=r4["LockToken"],
        Addresses=["0.0.0.0/0"],
    )
    print(f"WAF IPv4 set {WAF_IP_SET_NAME}: blocked all")

    r6 = waf.get_ip_set(Name=WAF_IP_SET_V6_NAME, Scope="CLOUDFRONT", Id=WAF_IP_SET_V6_ID)
    waf.update_ip_set(
        Name=WAF_IP_SET_V6_NAME,
        Scope="CLOUDFRONT",
        Id=WAF_IP_SET_V6_ID,
        LockToken=r6["LockToken"],
        Addresses=["::/0"],
    )
    print(f"WAF IPv6 set {WAF_IP_SET_V6_NAME}: blocked all")


def _disable_cloudfront(dist_id):
    cf = boto3.client("cloudfront")
    r = cf.get_distribution_config(Id=dist_id)
    config = r["DistributionConfig"]
    if not config["Enabled"]:
        print(f"CloudFront {dist_id}: already disabled")
        return
    config["Enabled"] = False
    cf.update_distribution(Id=dist_id, DistributionConfig=config, IfMatch=r["ETag"])
    print(f"CloudFront {dist_id}: disabled (propagating ~15 min)")
