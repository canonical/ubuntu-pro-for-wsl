#!/bin/bash
set -eu

cd ${TARGET_MODULE}
current_commit=$(git rev-parse HEAD~0)

# Find internal dependencies
repo="github.com/${GITHUB_REPOSITORY}"
url="${repo}/${TARGET_MODULE}"
regex="s#${url} \(${repo}/[^@]\+@v.*\)#\1#p"
dependencies=`go mod graph | sed -n "${regex}"`

for dependency in ${dependencies}; do  
    pattern="^${repo}/\(.*\)@v\([^-]\+\)-\([^-]\+\)-\(.*\)$"
    # Capture groups: (https://go.dev/ref/mod#versions)
    # \1 Dependency path
    # \2 Base version prefix
    # \3 Timestamp
    # \4 Revision identifier (12-character prefix of the commit hash)

    dep_path=`echo $dependency | sed "s#${pattern}#\1#"`
    dep_commit=`echo $dependency | sed "s#${pattern}#\4#"`
    
    diff_files="$(git diff --name-only ${dep_commit} -- "../${dep_path}")"
    if [ -z "${diff_files}" ] ; then
        continue
    fi

    echo "::group::Updating dependency ${dep_path}::"

    echo "Files changed since commit ${dep_commit}:"
    echo "${diff_files}"

    go get "${repo}/${dep_path}@${current_commit}"

    echo "::endgroup::"
done
