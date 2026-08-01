#!/usr/bin/env -f awk
# docs-freshness — verify intra-docs links and "path:" fences in
# the architecture index. POSIX awk is sufficient; the script
# has no external dependencies so it runs in any CI runner.
#
# Exit status:
#   0 — every link and path reference resolves
#   1 — at least one reference is broken
BEGIN {
    exit_code = 0
}

/^\[[^]]+\]\(/ {
    # Extract the link target out of the markdown [text](link).
    pos = index($0, "](")
    if (pos == 0) next
    rest = substr($0, pos + 2)
    end = index(rest, ")")
    if (end == 0) next
    link = substr(rest, 1, end - 1)
    if (link ~ /^https?:/ || link ~ /^#/) next
    target = link
    sub(/#.*/, "", target)
    if (target == "") next
    cmd = "test -e " target " || test -e docs/" target
    if (system(cmd) != 0) {
        print FILENAME ":" NR ": broken link " link
        exit_code = 1
    }
}

/^path:[[:space:]]+/ {
    sub(/^path:[[:space:]]+/, "")
    if (system("test -e " $0) != 0) {
        print FILENAME ":" NR ": missing path " $0
        exit_code = 1
    }
}

END {
    exit exit_code
}
