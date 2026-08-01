#!/usr/bin/env bash
# scripts/add-spdx-headers.sh
#
# Add SPDX-License-Identifier: Apache-2.0 to every Go source file
# that doesn't already have one. Idempotent; safe to re-run.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Files we touch: every .go under promptsheon/, cmd/, sdk/
# plus tooling in tools/.
mapfile -t files < <(find promptsheon cmd sdk tools -name '*.go' 2>/dev/null)

count=0
for f in "${files[@]}"; do
    if grep -q 'SPDX-License-Identifier:' "$f"; then
        continue
    fi
    # Skip vendored and generated files
    case "$f" in
        *_generated.go|*/vendor/*|*/.git/*) continue ;;
    esac
    # Insert as the first non-blank line. Use a temp file so
    # the rename is atomic on the same filesystem.
    tmp=$(mktemp)
    {
        # First line: shebang or package decl detection
        first=$(head -n1 "$f")
        if [[ "$first" == "package "* ]]; then
            echo '// SPDX-License-Identifier: Apache-2.0'
            cat "$f"
        else
            # Shebang or comment block above package decl
            echo '// SPDX-License-Identifier: Apache-2.0'
            echo ""
            cat "$f"
        fi
    } > "$tmp"
    mv "$tmp" "$f"
    count=$((count + 1))
done

echo "added SPDX header to $count files"
