#!/usr/bin/env bash
# Open, update, or close one GitHub issue with a fixed title.
# Usage: upsert-issue.sh TITLE BODY_FILE open|clear
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: upsert-issue.sh TITLE BODY_FILE open|clear" >&2
  exit 1
fi

title=$1
body=$2
mode=$3

if [ -z "$title" ] || [[ "$title" == *$'\n'* ]]; then
  echo "bad title" >&2
  exit 1
fi
if [ "$mode" != "open" ] && [ "$mode" != "clear" ]; then
  echo "bad mode" >&2
  exit 1
fi
if [ ! -f "$body" ]; then
  echo "missing body file" >&2
  exit 1
fi

number=$(
  gh issue list --state open --limit 30 --json number,title |
    TITLE="$title" python3 -c '
import json, os, sys
want = os.environ["TITLE"]
for item in json.load(sys.stdin):
    if item.get("title") == want:
        print(item["number"])
        break
'
)

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
head -c 60000 "$body" > "$tmp"

if [ "$mode" = "clear" ]; then
  if [ -n "$number" ]; then
    gh issue close "$number" --comment "Cleared."
  fi
  exit 0
fi

if [ -n "$number" ]; then
  gh issue edit "$number" --body-file "$tmp"
else
  gh issue create --title "$title" --body-file "$tmp"
fi
