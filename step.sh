#!/bin/bash
set -ex

# Ensure Microsoft AppCenter CLI installed
if hash appcenter 2>/dev/null; then
  echo "Microsoft AppCenter CLI already installed."
else
  echo "Microsoft AppCenter CLI is not installed. Installing..."
  npm install -g appcenter-cli
fi

envman add --key BITRISE_MAPPING_PATH
envman add --key BITRISE_APK_PATH
envman add --key BITRISE_APPCENTER_DIST_GROUP

appcenter distribute release -g "$dist_group" -f "$apk_path" -a $app_id --token $api_token
# waiting for AppCenter to support uploading symbols for Android
# see: https://github.com/microsoft/appcenter-cli/issues/551
#
# appcenter crashes upload-symbols -s "$mapping_path" -a $app_id --token $api_token

exit 0