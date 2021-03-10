#!/bin/bash
set -eo pipefail

tag="latest"

while getopts v:t:p: option
do
case "${option}"
in
v) version=${OPTARG};;
t) token=${OPTARG};;
p)
  if [[ "$OPTARG" == "true" ]]; 
  then
    tag="next"
  fi
  ;;
esac
done

[[ -z "$version" ]] && { echo "Please specify a new version, e.g., -v v1.0.0" ; exit 1; }
[[ -z "$token" ]] && { echo "Please specify an NPM auth token, e.g., -t mytoken" ; exit 1; }

wd="$(pwd -P)"

js_paths=()
while IFS=  read -r -d $'\0'; do
  js_paths+=("$REPLY")
done < <(find . -path "*/pb/javascript" ! -path "*/node_modules/*" -print0)

echo installing dependencies
npm install -g json >/dev/null 2>&1

for path in "${js_paths[@]}"; do
  cd "${path}"
  json -I -f package.json -e "this.version=('$version').replace('v', '')" >/dev/null 2>&1
  echo publishing js-protos in "${path}" with version "${version}" and token "${token}"
  NODE_AUTH_TOKEN="${token}" npm publish --access=public --tag ${tag}
  json -I -f package.json -e "this.version=('0.0.0')" >/dev/null 2>&1
  cd "${wd}"
done
