#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

# ── Go Boilerplate — Work Firewall ──────────────────────────────
# Based on: https://github.com/anthropics/claude-code/blob/main/.devcontainer
# ────────────────────────────────────────────────────────────────

# 1. Extract Docker DNS info BEFORE any flushing
DOCKER_DNS_RULES=$(iptables-save -t nat | grep "127\.0\.0\.11" || true)

# Flush existing rules and delete existing ipsets
iptables -F
iptables -X
iptables -t nat -F
iptables -t nat -X
iptables -t mangle -F
iptables -t mangle -X
ipset destroy allowed-domains 2>/dev/null || true

# 2. Selectively restore ONLY internal Docker DNS resolution
if [ -n "$DOCKER_DNS_RULES" ]; then
    echo "Restoring Docker DNS rules..."
    iptables -t nat -N DOCKER_OUTPUT 2>/dev/null || true
    iptables -t nat -N DOCKER_POSTROUTING 2>/dev/null || true
    echo "$DOCKER_DNS_RULES" | xargs -L 1 iptables -t nat
else
    echo "No Docker DNS rules to restore"
fi

# Allow DNS, SSH, localhost
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A INPUT -p udp --sport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 22 -j ACCEPT
iptables -A INPUT -p tcp --sport 22 -m state --state ESTABLISHED -j ACCEPT
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Create ipset with CIDR support
ipset create allowed-domains hash:net

# ── GitHub IP ranges ──────────────────────────────────────────────
echo "Fetching GitHub IP ranges..."
gh_ranges=$(curl -s https://api.github.com/meta)
if [ -z "$gh_ranges" ]; then
    echo "ERROR: Failed to fetch GitHub IP ranges"
    exit 1
fi

if ! echo "$gh_ranges" | jq -e '.web and .api and .git' >/dev/null; then
    echo "ERROR: GitHub API response missing required fields"
    exit 1
fi

echo "Processing GitHub IPs (IPv4 only)..."
while read -r cidr; do
    # Skip IPv6 ranges — ipset hash:net only supports IPv4
    [[ "$cidr" == *:* ]] && continue
    if [[ ! "$cidr" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}/[0-9]{1,2}$ ]]; then
        echo "ERROR: Invalid CIDR range from GitHub meta: $cidr"
        exit 1
    fi
    ipset add allowed-domains "$cidr" -exist
done < <(echo "$gh_ranges" | jq -r '(.web + .api + .git)[]' | sort -u)

# ── Required domains ──────────────────────────────────────────────
REQUIRED_DOMAINS=(
    # Claude Code
    "api.anthropic.com"
    "sentry.io"
    "statsig.anthropic.com"
    "statsig.com"
    "registry.npmjs.org"
    # VS Code
    "marketplace.visualstudio.com"
    "vscode.blob.core.windows.net"
    "update.code.visualstudio.com"
    # Go modules
    "proxy.golang.org"
    "sum.golang.org"
    "storage.googleapis.com"
    # Kibana (Appmax)
    "appmax-aws-max.kb.us-east-1.aws.found.io"
    # Docker Hub (for docker-compose: postgres, redis)
    "registry-1.docker.io"
    "auth.docker.io"
    "production.cloudflare.docker.com"
)

for domain in "${REQUIRED_DOMAINS[@]}"; do
    echo "Resolving $domain..."
    ips=$(dig +noall +answer A "$domain" | awk '$4 == "A" {print $5}')
    if [ -z "$ips" ]; then
        echo "ERROR: Failed to resolve $domain"
        exit 1
    fi
    while read -r ip; do
        if [[ ! "$ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            echo "ERROR: Invalid IP from DNS for $domain: $ip"
            exit 1
        fi
        ipset add allowed-domains "$ip" -exist
    done < <(echo "$ips")
done

# ── Optional domains (VPN/internal) ──────────────────────────────
OPTIONAL_DOMAINS=(
    "go-boilerplate.sandboxappmax.internal"
)

for domain in "${OPTIONAL_DOMAINS[@]}"; do
    echo "Resolving optional domain $domain..."
    ips=$(dig +noall +answer A "$domain" 2>/dev/null | awk '$4 == "A" {print $5}')
    if [ -z "$ips" ]; then
        echo "WARNING: Could not resolve $domain (VPN off?) — skipping"
        continue
    fi
    while read -r ip; do
        if [[ ! "$ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            echo "WARNING: Invalid IP from DNS for $domain: $ip — skipping"
            continue
        fi
        ipset add allowed-domains "$ip" -exist
    done < <(echo "$ips")
done

# ── Host network (Postgres, Redis on host) ────────────────────────
HOST_IP=$(ip route | grep default | cut -d" " -f3)
if [ -z "$HOST_IP" ]; then
    echo "ERROR: Failed to detect host IP"
    exit 1
fi
HOST_NETWORK=$(echo "$HOST_IP" | sed "s/\.[0-9]*$/.0\/24/")
echo "Host network detected as: $HOST_NETWORK"

iptables -A INPUT -s "$HOST_NETWORK" -j ACCEPT
iptables -A OUTPUT -d "$HOST_NETWORK" -j ACCEPT

# ── Docker-in-Docker network (172.17.0.0/16 for compose services) ─
iptables -A INPUT -s 172.17.0.0/16 -j ACCEPT
iptables -A OUTPUT -d 172.17.0.0/16 -j ACCEPT
iptables -A INPUT -s 172.18.0.0/16 -j ACCEPT
iptables -A OUTPUT -d 172.18.0.0/16 -j ACCEPT

# ── Default-deny policy ──────────────────────────────────────────
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP

iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT
iptables -A OUTPUT -j REJECT --reject-with icmp-admin-prohibited

# ── Verification ──────────────────────────────────────────────────
echo ""
echo "Firewall configuration complete [go-boilerplate]"
echo "Verifying..."

if curl --connect-timeout 5 https://example.com >/dev/null 2>&1; then
    echo "FAIL: example.com should be blocked"; exit 1
else
    echo "PASS: example.com blocked"
fi

if ! curl --connect-timeout 5 https://api.github.com/zen >/dev/null 2>&1; then
    echo "FAIL: api.github.com should be reachable"; exit 1
else
    echo "PASS: api.github.com reachable"
fi

if ! curl --connect-timeout 5 -s https://proxy.golang.org >/dev/null 2>&1; then
    echo "FAIL: proxy.golang.org should be reachable"; exit 1
else
    echo "PASS: proxy.golang.org reachable"
fi

echo ""
echo "Sandbox ready. Run: claude --dangerously-skip-permissions"
