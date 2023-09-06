#!/bin/bash
set -eu

repo="github.com/${GITHUB_REPOSITORY}"

# Gather all go modules
modules=$(go work edit -json                                \
    | jq -r '.Use | keys[] as $k | "\(.[$k].DiskPath)"'     \
    | sed $'s#\.\/##'                                       \
)

# Find internal dependencies
for mod in ${modules} ; do
    cd "${mod}"
    echo "::group::Updating module ${mod}::"
    
    url="${repo}/${mod}"
    regex="s#${url} \(${repo}/[^@]\+\)@v.*#\1#p"
    dependencies=`go mod graph | sed -n "${regex}"`
    
    for dep in ${dependencies}; do
        go get "${dep}@main"
    done

    go mod tidy
    
    echo "Done"
    echo "::endgroup::"
    cd ~-
done

go work sync