# kms-import

CLI tool and Go library that imports a GitHub App private key (PEM) into AWS KMS
as non-extractable key material.

## Verifying releases

Release artifacts are published to [GitHub Releases](https://github.com/chinmina/kms-import/releases)
and signed with [Cosign](https://docs.sigstore.dev/cosign/overview/) using
keyless signing (Sigstore OIDC) — there is no long-lived signing key to manage.
Every artifact (each platform archive and the `checksums.txt` file) has a
matching `<artifact>.cosign.bundle` containing its signature and certificate.

To verify a downloaded artifact, install [`cosign`](https://docs.sigstore.dev/cosign/system_config/installation/)
and run `verify-blob`, asserting the identity that produced the signature — the
release workflow running on a tag in this repository:

```sh
# e.g. ARTIFACT=kms-import_linux_amd64.tar.gz and TAG=v1.0.0
cosign verify-blob \
  --bundle "${ARTIFACT}.cosign.bundle" \
  --certificate-identity "https://github.com/chinmina/kms-import/.github/workflows/release.yml@refs/tags/${TAG}" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "${ARTIFACT}"
```

A `Verified OK` result confirms the artifact was produced and signed by this
repository's release workflow for that tag. Verify `checksums.txt` the same way,
then check your archive against it with `sha256sum --check checksums.txt`.
