#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
artifact_dir="$repo_dir/dist"
bundle_dir=$(mktemp -d "${TMPDIR:-/tmp}/rdl-mcpb.XXXXXX")
trap 'rm -rf "$bundle_dir"' EXIT HUP INT TERM

mkdir -p "$bundle_dir/server"
cp "$repo_dir/mcpb/manifest.json" "$bundle_dir/manifest.json"
cp "$repo_dir/mcpb/rdl-mcp" "$bundle_dir/server/rdl-mcp"
cp "$repo_dir/mcpb/rdl-mcp.ps1" "$bundle_dir/server/rdl-mcp.ps1"
cp "$artifact_dir"/rdl-mcp_* "$bundle_dir/server/"

npm exec --yes --package=@anthropic-ai/mcpb -- mcpb validate "$bundle_dir"
npm exec --yes --package=@anthropic-ai/mcpb -- mcpb pack "$bundle_dir" "$artifact_dir/rdl-mcp.mcpb"
