# kms-import

CLI tool and Go library that imports a GitHub App private key (PEM) into AWS KMS
as non-extractable key material.

A GitHub App's private key is powerful: anyone who holds it can impersonate the
app indefinitely. Stored as a plaintext secret, that key is a standing
liability — every place it is copied is another way to lose it.

Importing the key into AWS KMS removes that attack surface entirely. The key
material lives inside the KMS HSM, where it cannot be read back out; the
application signs GitHub App JWTs through the KMS `Sign` API and never touches
the key itself. The catch is that importing key material into KMS is a fiddly,
cryptographically-unforgiving sequence — which is why the posture is so often
skipped. `kms-import` reduces it to a single command.

Once the key material is in KMS, no other copy of it needs to exist anywhere.
Import it, confirm it, and destroy your local PEM: the canonical private key now
lives only inside KMS, governed by IAM, audited by CloudTrail, and rotatable by
a single alias update.

## Quick start

`kms-import` does not create KMS keys; it only pushes key material into one that
already exists with `EXTERNAL` origin. Create a compatible key, import your
GitHub App PEM into it, then throw the PEM away.

1. **Install the binary** (see [Installation](#installation) for all the
   options, including verified release downloads):

   ```sh
   mise use -g github:chinmina/kms-import
   ```

2. **Create a compatible KMS key.** It must have `EXTERNAL` origin (so its
   material can be imported), key spec `RSA_2048` (the spec GitHub issues App
   keys in), and `SIGN_VERIFY` usage (the key signs JWTs; it does not encrypt):

   ```sh
   aws kms create-key --origin EXTERNAL --key-spec RSA_2048 --key-usage SIGN_VERIFY
   ```

   This is the bare minimum. In production you would provision the key — and its
   alias, tags, and resource policy — with CloudFormation or Terraform. Do that,
   but **import the key material with `kms-import`, not with your IaC.**
   Terraform's `aws_kms_external_key` takes the raw key material and persists it:
   "all arguments including the key material will be stored in the raw state as
   plain text" ([provider docs][tf-state]). That recreates exactly the extra
   plaintext copy of the private key this approach exists to eliminate.
   (CloudFormation cannot import external key material at all.)

3. **Import the key material** into the key by ID or ARN:

   ```sh
   kms-import --key-file app.pem --key-arn arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
   ```

   On success it prints the resolved key ID and resulting state:

   ```text
   Imported key material — key ID: 1234abcd-12ab-34cd-56ef-1234567890ab, state: Enabled
   ```

4. **Destroy the local PEM.** The key material now lives only in KMS. Grant the
   application `kms:Sign` on the key and delete every other copy of the PEM.

[tf-state]: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/kms_external_key

## Installation

Pre-built binaries for Linux, macOS, and Windows (amd64/arm64) are published to
[GitHub Releases](https://github.com/chinmina/kms-import/releases). Every
artifact carries a build-provenance attestation — see
[Verifying releases](#verifying-releases).

<details>
<summary><strong>mise (recommended)</strong></summary>

[mise](https://mise.jdx.dev/) installs directly from GitHub Releases via its
[GitHub backend](https://mise.jdx.dev/dev-tools/backends/github.html), verifying
the artifact's checksum and build-provenance attestation as part of the install:

```sh
mise use -g github:chinmina/kms-import
```

Or pin a version for a project in its `mise.toml`:

```toml
[tools]
"github:chinmina/kms-import" = "1.0.0"
```

</details>

<details>
<summary><strong>Install script</strong></summary>

Each release includes a self-contained installer script (generated with
[binstaller](https://github.com/binary-install/binstaller)) that detects your
platform and verifies the download against the release checksums embedded in
the script — no network fetch of the checksum file:

```sh
curl -fsSL https://github.com/chinmina/kms-import/releases/latest/download/install.sh | sh
```

It installs to `~/.local/bin` by default; pass `-b` for a different directory
and an optional tag to pin a version:

```sh
curl -fsSL https://github.com/chinmina/kms-import/releases/latest/download/install.sh \
  | sh -s -- -b /usr/local/bin v1.0.0
```

The script itself carries a build-provenance attestation, so instead of piping
straight to `sh` you can prove it was produced by this repository's release
workflow first (requires an authenticated [GitHub CLI](https://cli.github.com/)):

```sh
curl -fsSL -O https://github.com/chinmina/kms-import/releases/latest/download/install.sh
gh attestation verify install.sh --repo chinmina/kms-import
sh install.sh
```

</details>

<details>
<summary><strong>Manual download</strong></summary>

Download the archive for your platform from the
[releases page](https://github.com/chinmina/kms-import/releases), verify its
provenance, and put the binary on your `PATH`:

```sh
OS=linux ARCH=amd64   # or darwin/windows, arm64
curl -fsSLO "https://github.com/chinmina/kms-import/releases/latest/download/kms-import_${OS}_${ARCH}.tar.gz"
gh attestation verify "kms-import_${OS}_${ARCH}.tar.gz" --repo chinmina/kms-import
tar -xzf "kms-import_${OS}_${ARCH}.tar.gz" kms-import
install -m 0755 kms-import ~/.local/bin/
```

Windows archives are `.zip`. See [Verifying releases](#verifying-releases) for
what the attestation proves and for checksum-only verification.

</details>

<details>
<summary><strong>go install</strong></summary>

Build from source with Go 1.26+:

```sh
go install github.com/chinmina/kms-import/cmd/kms-import@latest
```

Source builds are not stamped with a release version, so `--version` reports
`dev`.

</details>

## Usage

The [Quick start](#quick-start) shows a basic invocation. Under the hood
`kms-import` reads the PEM, converts it to PKCS#8 DER, fetches wrapping
parameters from KMS, encrypts the key material under the returned wrapping key,
and calls `ImportKeyMaterial`. The full flag set follows.

### CLI reference

Exactly one target flag (`--key-id` or `--key-arn`) is required; supplying both,
or neither, is an error. All other flags are optional except `--key-file`.

| Flag         | Type   | Default | Description |
|--------------|--------|---------|-------------|
| `--key-file` | string | —       | **Required.** Path to the PEM-encoded private key file. Accepts `BEGIN RSA PRIVATE KEY` (PKCS#1) and `BEGIN PRIVATE KEY` (PKCS#8); any other header is an error. |
| `--key-id`   | string | —       | Target KMS key ID. Mutually exclusive with `--key-arn`. |
| `--key-arn`  | string | —       | Target KMS key ARN. Mutually exclusive with `--key-id`. |
| `--expires`  | string | none (key material does not expire) | Expiry for the imported key material as an RFC 3339 timestamp (e.g. `2027-01-01T00:00:00Z`). A malformed or already-passed value is rejected before any AWS call is made. |
| `--profile`  | string | SDK default | AWS named profile to use. |
| `--region`   | string | SDK default | AWS region to use. |
| `--json`     | bool   | `false` | Emit the result as a single JSON object on stdout and suppress all other output. |
| `--help`, `-h`    | bool | `false` | Show help. |
| `--version`, `-v` | bool | `false` | Print the version. |

**Target selection.** Exactly one of `--key-id` / `--key-arn` must be given
(mutually exclusive). The KMS import APIs accept only a key ID or key ARN — not
an alias — so there is no `--alias` flag; see the rotation workflow below for how
aliases fit in.

**Credentials.** Credentials and region resolve through the standard AWS SDK
chain (environment variables, shared config/credentials files, IMDS). `--profile`
and `--region` override the corresponding defaults; when omitted, the SDK
defaults apply.

**Expiry and reimport.** By default the imported material does not expire
(`KEY_MATERIAL_DOES_NOT_EXPIRE`). With `--expires`, the material is set to expire
at the supplied time (`KEY_MATERIAL_EXPIRES` + `ValidTo`). The same invocation
performs both the initial import and a reimport after material has expired or
been deleted — there is no separate mode or flag. AWS requires a reimport to use
the *same* key material as the original.

**JSON output.** With `--json`, stdout carries only the result object:

```json
{"keyId":"1234abcd-12ab-34cd-56ef-1234567890ab","keyState":"Enabled"}
```

**Errors.** Any AWS API failure exits non-zero with the error written to stderr;
stdout stays empty (including in `--json` mode). Input errors — unreadable
key file, undecodable PEM, unsupported PEM header, invalid `--expires` — also
exit non-zero with a descriptive message.

### Wrapping algorithm

The wrapping algorithm is fixed at `RSA_AES_KEY_WRAP_SHA_256` with a `RSA_4096`
wrapping key and is not configurable. This is AWS's current recommendation for
importing asymmetric private key material; it avoids the size constraints that
affect the pure `RSAES_OAEP_*` algorithms with RSA key material. Only RSA 2048
key material (the GitHub App key spec) is supported.

### Library usage

The import logic is exposed as a Go library that builds no SDK client and never
calls `os.Exit`; the caller injects a `KMSClient` (the AWS SDK `*kms.Client`
satisfies it). The CLI is also mountable as a subcommand of another urfave/cli
v3 application via `cli.Command()`.

```go
import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/chinmina/kms-import/pkg/kmsimport"
)

cfg, _ := config.LoadDefaultConfig(ctx)
der, _ := kmsimport.KeyMaterialFromPEM(pemBytes) // PEM (PKCS#1 or PKCS#8) → PKCS#8 DER
res, err := kmsimport.Import(ctx,
	kmsimport.WithClient(kms.NewFromConfig(cfg)),
	kmsimport.WithKeyID("1234abcd-12ab-34cd-56ef-1234567890ab"),
	kmsimport.WithKeyMaterial(der),
)
```

## Security rationale

Storing a GitHub App private key in KMS rather than as a plaintext secret is a
materially stronger posture:

- **Non-extractability.** Imported key material lives inside the KMS HSM
  boundary. There is no API to read it back, it never appears in plaintext
  outside the HSM, and no AWS operator can retrieve it. The application calls
  `kms:Sign` and receives only signature bytes — the key itself never moves.
- **IAM and resource-policy access control.** Every use of the key is gated by
  both the caller's IAM policy and the KMS key's resource policy. Access is
  least-privilege and centrally revocable without touching the application, in
  contrast to a raw secret that grants full impersonation to anyone who can read
  it.
- **CloudTrail audit visibility.** KMS records every `GetParametersForImport`,
  `ImportKeyMaterial`, and subsequent `Sign` call in CloudTrail, giving a
  complete, tamper-evident audit trail of who used the key and when. A plaintext
  secret read leaves no comparable record of subsequent use.
- **Alias-based rotation.** Referencing the key through a KMS alias makes
  rotation a single pointer update (see below) with no application restart or
  configuration change, and no window where the old and new keys are both live
  in application config.

For operational *revocation* of a compromised key, revoke the GitHub App private
key in GitHub directly — that is faster and more direct than waiting for KMS key
material to expire. The `--expires` flag exists for organisational compliance
requirements, not for emergency revocation.

## IAM permissions

`kms-import` needs exactly two KMS actions: `kms:GetParametersForImport` and
`kms:ImportKeyMaterial`. Because KMS authorizes key operations through the key's
resource policy, the importing principal must be granted these actions in
**both** the caller's IAM policy and the KMS key's resource policy. The two
fragments are separated below.

Replace the account ID, region, key ID, and role ARN with your own values.

### Caller IAM policy

Attach this to the principal (role/user) that runs `kms-import`. It is scoped to
the specific key ARN. The `GetParametersForImport` statement is additionally
constrained to the wrapping algorithm and key spec this tool uses; the
`ImportKeyMaterial` statement may optionally be constrained with
`kms:ExpirationModel` / `kms:ValidTo` if you want to mandate (non-)expiry.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "KMSGetWrappingParameters",
      "Effect": "Allow",
      "Action": "kms:GetParametersForImport",
      "Resource": "arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
      "Condition": {
        "StringEquals": {
          "kms:WrappingAlgorithm": "RSA_AES_KEY_WRAP_SHA_256",
          "kms:WrappingKeySpec": "RSA_4096"
        }
      }
    },
    {
      "Sid": "KMSImportKeyMaterial",
      "Effect": "Allow",
      "Action": "kms:ImportKeyMaterial",
      "Resource": "arn:aws:kms:us-east-1:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"
    }
  ]
}
```

### KMS key resource policy

Add this statement to the resource policy of the target KMS key, granting the
importing principal the same two actions. Within a key policy, `"Resource": "*"`
refers to the key the policy is attached to.

```json
{
  "Sid": "AllowKeyMaterialImport",
  "Effect": "Allow",
  "Principal": {
    "AWS": "arn:aws:iam::111122223333:role/KeyImporter"
  },
  "Action": [
    "kms:GetParametersForImport",
    "kms:ImportKeyMaterial"
  ],
  "Resource": "*"
}
```

> **Note on alias scoping.** The `kms:RequestAlias` condition key — which scopes
> access to requests that name the key by a particular alias — applies only to
> cryptographic operations, `DescribeKey`, and `GetPublicKey`. It has **no
> effect** on `GetParametersForImport` or `ImportKeyMaterial`; a policy that
> tried to scope these import actions by alias would never match and the
> permission would be ineffective. Scope the import permissions by the key ARN
> instead, as shown above. (Alias scoping is still the right pattern for the
> *runtime* `kms:Sign` permission the application uses.)

## Alias-based rotation workflow

The recommended production pattern is to always reference the KMS key through an
alias, so the application never needs to know the underlying key ID. To rotate
to a new GitHub App private key:

1. **Create a new KMS key** with `EXTERNAL` origin via your IaC (CDK/Terraform).
   Do not point the alias at it yet.
2. **Import the new private key** into the new key by its ID or ARN:

   ```sh
   kms-import --key-file new-app.pem --key-arn arn:aws:kms:us-east-1:111122223333:key/<new-key-id>
   ```

   Import always targets a specific key generation by ID or ARN — the alias
   still points at the old key at this stage, and the import APIs do not accept
   an alias anyway.
3. **Verify** the new key reached `Enabled` (the `kms-import` confirmation, or
   `aws kms describe-key`).
4. **Update the alias** to point at the new key (via IaC or
   `aws kms update-alias`). The application picks up the new key on its next
   signing call — no restart, redeploy, or configuration change required.
5. **Retire the old key** once you are satisfied the new key is in use.

Because signing always goes through the alias, the cutover is atomic from the
application's perspective and requires no downtime.

## Verifying releases

Release artifacts — the binary archives and the generated `install.sh` — are
published to [GitHub Releases](https://github.com/chinmina/kms-import/releases)
and carry a [build-provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations/using-artifact-attestations-to-establish-provenance-for-builds)
(SLSA) generated by the release workflow with [Sigstore](https://www.sigstore.dev/)
keyless signing — there is no long-lived signing key. Each artifact is bound, by
digest, to the source commit and the workflow that produced it.

To verify a downloaded artifact, install the [GitHub CLI](https://cli.github.com/)
(≥ 2.49.0) and run:

```sh
# e.g. ARTIFACT=kms-import_linux_amd64.tar.gz
gh attestation verify "$ARTIFACT" --repo chinmina/kms-import
```

A successful result prints the provenance summary (repository, commit, and the
release workflow that built it). To additionally pin the signing workflow:

```sh
gh attestation verify "$ARTIFACT" --repo chinmina/kms-import \
  --signer-workflow chinmina/kms-import/.github/workflows/release.yml
```

The attestation is a Sigstore bundle, so [`cosign`](https://docs.sigstore.dev/)
can verify it too. `checksums.txt` is still published — check your archive
against it with `sha256sum --check checksums.txt`.
