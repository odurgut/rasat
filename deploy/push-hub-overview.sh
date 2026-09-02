#!/usr/bin/env bash
# Update Docker Hub short description + Overview from deploy/docker-hub.md.
# Needs DOCKERHUB_USERNAME and DOCKERHUB_TOKEN (PAT: Read, Write, Delete).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
README="$ROOT/deploy/docker-hub.md"
SHORT="Self-hosted observability for OpenTelemetry traces, logs, and a service map"

if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
  echo "DOCKERHUB_USERNAME and DOCKERHUB_TOKEN are required" >&2
  exit 1
fi

BODY="$(jq -n \
  --arg description "$SHORT" \
  --rawfile full_description "$README" \
  '{description:$description, full_description:$full_description}')"

JWT="$(curl -fsS -X POST https://hub.docker.com/v2/users/login/ \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg u "$DOCKERHUB_USERNAME" --arg p "$DOCKERHUB_TOKEN" '{username:$u,password:$p}')" \
  | jq -r .token)"

if [[ -z "$JWT" || "$JWT" == "null" ]]; then
  echo "Docker Hub login did not return a token" >&2
  exit 1
fi

patch() {
  local url="$1"
  local auth="$2"
  curl -sS -o /tmp/hub-overview.json -w '%{http_code}' -X PATCH "$url" \
    -H "Authorization: Bearer $auth" \
    -H 'Content-Type: application/json' \
    -d "$BODY"
}

# New Hub API first. The legacy /v2/repositories/{user}/{repo}/ PATCH is 403
# for JWTs minted from a personal access token.
CODE="$(patch "https://hub.docker.com/v2/namespaces/${DOCKERHUB_USERNAME}/repositories/rasat" "$JWT")"
if [[ "$CODE" == "200" || "$CODE" == "202" ]]; then
  echo "Updated Hub overview (namespaces API)"
  exit 0
fi

CODE2="$(patch "https://hub.docker.com/v2/namespaces/${DOCKERHUB_USERNAME}/repositories/rasat" "$DOCKERHUB_TOKEN")"
if [[ "$CODE2" == "200" || "$CODE2" == "202" ]]; then
  echo "Updated Hub overview (PAT as bearer)"
  exit 0
fi

echo "Hub overview PATCH failed (${CODE}, then ${CODE2}). Token needs Read, Write, and Delete." >&2
cat /tmp/hub-overview.json >&2 || true
echo >&2
exit 1
