#!/bin/bash
set -ex

# Ensure Microsoft AppCenter CLI installed
if hash appcenter 2>/dev/null; then
  echo "Microsoft AppCenter CLI already installed."
else
  echo "Microsoft AppCenter CLI is not installed. Installing..."
  npm install -g appcenter-cli
fi

envman add --key BITRISE_DSYM_PATH
envman add --key BITRISE_IPA_PATH
envman add --key BITRISE_APPCENTER_DIST_GROUP

appcenter distribute release -g "$dist_group" -f "$ipa_path" --token $api_token 
appcenter crashes upload-symbols -s "$dsym_path" -a $app_id --token $api_token 

exit 0