#!/bin/bash
set -eu

github_repo="${1:-""}"
target_module="${2:-""}"

if [[ -z "${github_repo}" || -z "${target_module}" ]] ; then
    echo "Update the out-of-date workspace dependencies of the target modules"
    echo "This script must be run from the workspace root."
    echo "Usage:"
    echo "    $(basename $0) github_repo target_module"
    echo ""
    echo "github_repo:"
    echo "    The 'user/repository' of the workspace"
    echo ""
    echo "target_module:"
    echo "    The path of the module relative to the workspace root"
    exit 2
fi

cd ${target_module}
current_commit=$(git rev-parse HEAD~0)

# Find internal dependencies
repo="github.com/${github_repo}"
url="${repo}/${target_module}"
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

    export GIT_TERMINAL_PROMPT="0"
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
