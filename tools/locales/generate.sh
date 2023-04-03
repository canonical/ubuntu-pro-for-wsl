#!/bin/bash
set -eu

POT="po/ubuntu-pro.pot"
PO_DIR="po"
MO_DIR="generated"

# Monorepo management: finding root
REPO_ROOT=$(cd $(dirname $0)/../../.. && pwd)
cd "$REPO_ROOT"

# Generating locales
cd "$REPO_ROOT"
go run "$REPO_ROOT/common/i18n/generate/generate-locales.go" update-po "$POT" "$PO_DIR" $PACKAGES
go run "$REPO_ROOT/common/i18n/generate/generate-locales.go" generate-mo "$PO_DIR" "$MO_DIR"