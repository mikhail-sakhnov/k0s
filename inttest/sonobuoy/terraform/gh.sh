#!/usr/bin/env bash
set -e
sleep 30

echo "prepare environment"
sudo snap install jq
sudo snap install kubectl --classic

# Parse CLI args
readonly git_tag="v0.7.0-beta1"
readonly github_repo_owner="k0sproject"
readonly github_repo_name="k0s"
readonly release_asset_filename="k0s-v0.7.0-beta1-amd64"
readonly output_path="/usr/local/bin/k0s"

# Get the "github tag id" of this release
github_tag_id=$(curl --silent --show-error \
                     --request GET \
                     "https://api.github.com/repos/$github_repo_owner/$github_repo_name/releases" \
                     | jq --raw-output ".[] | select(.tag_name==\"$git_tag\").id")

# Get the download URL of our desired asset
download_url=$(curl --silent --show-error \
                    --header "Accept: application/vnd.github.v3.raw" \
                    --location \
                    --request GET \
                    "https://api.github.com/repos/$github_repo_owner/$github_repo_name/releases/$github_tag_id" \
                    | jq --raw-output ".assets[] | select(.name==\"$release_asset_filename\").url")

# Get GitHub's S3 redirect URL
# Why not just curl's built-in "--location" option to auto-redirect? Because curl then wants to include all the original
# headers we added for the GitHub request, which makes AWS complain that we're trying strange things to authenticate.
redirect_url=$(curl --silent --show-error \
          --header "Accept: application/octet-stream" \
          --request GET \
          --write-out "%{redirect_url}" \
          "$download_url")

# Finally download the actual binary
sudo curl --silent --show-error \
          --header "Accept: application/octet-stream" \
          --output "$output_path" \
          --request GET \
          "$redirect_url"

sudo chmod +x $output_path