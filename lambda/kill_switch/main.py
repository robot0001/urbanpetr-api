import json
import os

import boto3

API_LAMBDA_NAME = os.environ["API_LAMBDA_NAME"]
WAF_IP_SET_ID = os.environ["WAF_IP_SET_ID"]
WAF_IP_SET_NAME = os.environ["WAF_IP_SET_NAME"]
WAF_IP_SET_V6_ID = os.environ["WAF_IP_SET_V6_ID"]
WAF_IP_SET_V6_NAME = os.environ["WAF_IP_SET_V6_NAME"]
API_CF_DIST_ID = os.environ["API_CF_DIST_ID"]
WEBSITE_CF_DIST_ID = os.environ["WEBSITE_CF_DIST_ID"]
ADMIN_CF_DIST_ID = os.environ["ADMIN_CF_DIST_ID"]


def handler(event, context):
    print(f"Kill switch triggered. Event: {json.dumps(event)}")

    _throttle_api_lambda()
    _block_all_waf()
    _disable_cloudfront()

    return {"status": "killed"}


def _throttle_api_lambda():
    boto3.client("lambda", region_name="eu-central-1").put_function_concurrency(
        FunctionName=API_LAMBDA_NAME,
        ReservedConcurrentExecutions=0,
    )
    print(f"Lambda {API_LAMBDA_NAME}: concurrency set to 0")


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


def _disable_cloudfront():
    cf = boto3.client("cloudfront")
    for dist_id in [API_CF_DIST_ID, WEBSITE_CF_DIST_ID, ADMIN_CF_DIST_ID]:
        r = cf.get_distribution_config(Id=dist_id)
        config = r["DistributionConfig"]
        config["Enabled"] = False
        cf.update_distribution(Id=dist_id, DistributionConfig=config, IfMatch=r["ETag"])
        print(f"CloudFront {dist_id}: disabled (propagating ~15 min)")
