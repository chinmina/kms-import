# Black-box system test for kms-import.
#
# The BATS container invokes the released kms-import and the unshipped
# kms-support binaries mounted from dist/. It generates a fresh private key,
# provisions a compatible KMS key in Local KMS, and asserts that kms-import
# reports success.

setup() {
  load /usr/lib/bats/bats-support/load.bash
  load /usr/lib/bats/bats-assert/load.bash

  KMS_SUPPORT=/dist/kms-support
  KMS_IMPORT=/dist/kms-import
  export KMS_SUPPORT KMS_IMPORT

  AWS_ACCESS_KEY_ID=test
  AWS_SECRET_ACCESS_KEY=test
  AWS_REGION=us-east-1
  AWS_ENDPOINT_URL_KMS=http://kms:8080
  export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION AWS_ENDPOINT_URL_KMS

  TEST_KEY=$(mktemp -u)
  export TEST_KEY
}

teardown() {
  rm -f "$TEST_KEY"
}

# create_target_key keeps calling kms-support kms-create-key until Local KMS is
# ready. This is the system-test readiness probe; it proves the KMS API is
# reachable with the SDK configuration used by the product.
create_target_key() {
  local key_id=""
  for _ in {1..30}; do
    key_id=$("$KMS_SUPPORT" kms-create-key 2>/dev/null)
    if [[ -n "$key_id" ]]; then
      printf "%s" "$key_id"
      return 0
    fi
    sleep 1
  done
  return 1
}

@test "imports a freshly generated PKCS#8 key" {
  "$KMS_SUPPORT" generate-key --pkcs8 --output "$TEST_KEY"

  local key_id
  key_id=$(create_target_key)
  [[ -n "$key_id" ]]

  run "$KMS_IMPORT" --key-file "$TEST_KEY" --key-id "$key_id" --json
  assert_success
  assert_output --partial '"keyState":"Enabled"'
}

@test "imports a freshly generated PKCS#1 key" {
  "$KMS_SUPPORT" generate-key --pkcs1 --output "$TEST_KEY"

  local key_id
  key_id=$(create_target_key)
  [[ -n "$key_id" ]]

  run "$KMS_IMPORT" --key-file "$TEST_KEY" --key-id "$key_id" --json
  assert_success
  assert_output --partial '"keyState":"Enabled"'
}
