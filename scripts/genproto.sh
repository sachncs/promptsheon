#!/usr/bin/env bash
# genproto.sh regenerates the gRPC stubs in
# internal/pluginproto/pluginv1 from the .proto file in
# internal/pluginproto/proto. The script assumes protoc,
# protoc-gen-go, and protoc-gen-go-grpc are on PATH; install them
# with:
#
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#
# Re-run this script whenever the .proto file changes and commit
# the regenerated plugin.pb.go and plugin_grpc.pb.go.

set -euo pipefail

cd "$(dirname "$0")/.."

out_dir="internal/pluginproto/pluginv1"
mkdir -p "$out_dir"

protoc \
  --go_out="$out_dir" \
  --go_opt=paths=source_relative \
  --go-grpc_out="$out_dir" \
  --go-grpc_opt=paths=source_relative \
  --proto_path=internal/pluginproto/proto \
  internal/pluginproto/proto/plugin.proto

# protoc generates under a directory matching the go_package option;
# flatten it so the package source-of-truth lives directly in
# pluginv1/.
generated_dir="$out_dir/github.com/sachncs/promptsheon/internal/pluginproto/pluginv1"
if [ -d "$generated_dir" ]; then
  mv "$generated_dir"/*.go "$out_dir"/
  rmdir -p "$generated_dir" 2>/dev/null || true
fi

echo "regenerated $out_dir/{plugin,plugin_grpc}.pb.go"