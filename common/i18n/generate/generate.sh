#!/bin/bash
set -eu

DOMAIN="ubuntu-pro"
PO_DIR="po"
MO_DIR="generated"

# Monorepo management: finding root
REPO_ROOT=$(cd $(dirname $0)/../../.. && pwd)
cd "$REPO_ROOT"

# Monorepo management: finding modules that need localization
PACKAGES="`grep -or "github.com/canonical/ubuntu-pro-for-windows/common/i18n" ./* \
    2>/dev/null                                     \
    | grep ".go"                                    \
    | grep -v "_test.go"                            \
    | sed -n 's/\.\/\([^\/]\+\)\/.*/\1/p'           \
    | uniq`"

if [ "$PACKAGES" = "" ]; then
    echo "No packages have a dependency on i18n"
else
    echo "The following packages depend on i18n:"
    printf -- "- %s\n" $PACKAGES
fi

# Generating locales
cd "$REPO_ROOT"
go run "$REPO_ROOT/common/i18n/generate/generate-locales.go" update-po "$PO_DIR/$DOMAIN.pot" "$PO_DIR" $PACKAGES
go run "$REPO_ROOT/common/i18n/generate/generate-locales.go" generate-mo "$DOMAIN" "$PO_DIR" "$MO_DIR"