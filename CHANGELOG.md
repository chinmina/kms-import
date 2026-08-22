# Changelog

## [1.1.2](https://github.com/chinmina/kms-import/compare/v1.1.1...v1.1.2) (2026-08-22)


### Bug Fixes

* **deps:** configure renovate ([#28](https://github.com/chinmina/kms-import/issues/28)) ([c832729](https://github.com/chinmina/kms-import/commit/c832729609569d8bfdc06247f9040eb3db412c19))
* **deps:** update dependency go to v1.26.6 ([#29](https://github.com/chinmina/kms-import/issues/29)) ([fb57da2](https://github.com/chinmina/kms-import/commit/fb57da223f9dbb03084ad6f963aa4c4e9029eef9))
* **deps:** update github actions ([#30](https://github.com/chinmina/kms-import/issues/30)) ([85663e5](https://github.com/chinmina/kms-import/commit/85663e5da494c6ff010778d628760b8a25611d0f))
* **deps:** update github actions ([#35](https://github.com/chinmina/kms-import/issues/35)) ([385f253](https://github.com/chinmina/kms-import/commit/385f253998222681b839434d9a57a00b0e191d8e))
* **deps:** update go dependencies ([#32](https://github.com/chinmina/kms-import/issues/32)) ([5c07a78](https://github.com/chinmina/kms-import/commit/5c07a78652a799734eec5b52b4ac410f64f82192))
* **deps:** update mise dependencies ([#31](https://github.com/chinmina/kms-import/issues/31)) ([6842240](https://github.com/chinmina/kms-import/commit/6842240250d09546723bdd7f70d526177ca5025e))
* **deps:** update module github.com/urfave/cli/v3 to v3.11.0 ([#36](https://github.com/chinmina/kms-import/issues/36)) ([78f7073](https://github.com/chinmina/kms-import/commit/78f707339bc9554dc720d948688fc69fb0de72ef))

## [1.1.1](https://github.com/chinmina/kms-import/compare/v1.1.0...v1.1.1) (2026-07-17)


### Bug Fixes

* **docs:** record the [#24](https://github.com/chinmina/kms-import/issues/24) dependency upgrade in the changelog ([#25](https://github.com/chinmina/kms-import/issues/25)) ([6bfd8c8](https://github.com/chinmina/kms-import/commit/6bfd8c82244087ae1a0cc20235ca0175c5e28d27))

### Dependencies

* upgrade Go, modules, and GitHub Actions ([#24](https://github.com/chinmina/kms-import/issues/24)) ([fdb8ca2](https://github.com/chinmina/kms-import/commit/fdb8ca2c64e7ef77f8e620eabed8af28c40ed2f4))

## [1.1.0](https://github.com/chinmina/kms-import/compare/v1.0.0...v1.1.0) (2026-06-10)


### Features

* publish attested install script and document installation methods ([#20](https://github.com/chinmina/kms-import/issues/20)) ([6eb41cf](https://github.com/chinmina/kms-import/commit/6eb41cfaff3f2587db5e7061a0aa264f7a695e84))

## [1.0.0](https://github.com/chinmina/kms-import/compare/v0.1.0...v1.0.0) (2026-06-10)


### Bug Fixes

* make CLI help and code comments the durable reference; drop plan docs ([#18](https://github.com/chinmina/kms-import/issues/18)) ([099d832](https://github.com/chinmina/kms-import/commit/099d832a387da4f75d3cd3e49f291aa36e242f0a))

## 0.1.0 (2026-06-09)


### Features

* add --json output mode ([#8](https://github.com/chinmina/kms-import/issues/8)) ([6eabe7b](https://github.com/chinmina/kms-import/commit/6eabe7b4463be09b8405cd447ebc937ad9cdfe7f))
* add KMS import library core ([#2](https://github.com/chinmina/kms-import/issues/2)) ([f845945](https://github.com/chinmina/kms-import/commit/f845945ef5acf3346aa9ff7158a1a2b26f46ea20))
* automated attested releases ([#13](https://github.com/chinmina/kms-import/issues/13)) ([be36872](https://github.com/chinmina/kms-import/commit/be368726806ebd66c81820501ebb16230e328986))
* drop the --alias target flag ([#11](https://github.com/chinmina/kms-import/issues/11)) ([ea36f7f](https://github.com/chinmina/kms-import/commit/ea36f7fe7dd191ad8f074095dda137f2508d8631))
* key-material expiry support (Phase 7) ([#7](https://github.com/chinmina/kms-import/issues/7)) ([80f29a9](https://github.com/chinmina/kms-import/commit/80f29a9296efbc3da6821b8a39dbdd7312ce6c5c))
* PEM format coverage and input errors (Phase 5) ([#5](https://github.com/chinmina/kms-import/issues/5)) ([a81e8d2](https://github.com/chinmina/kms-import/commit/a81e8d2828d62914c4287eed45c17135d3a64fec))
* Phase 6 — KMS key identification + AWS config flags ([#6](https://github.com/chinmina/kms-import/issues/6)) ([4d00f08](https://github.com/chinmina/kms-import/commit/4d00f084e9b214ca323a1e614900ccca0d671183))
* runnable CLI to import GitHub App keys into AWS KMS ([#3](https://github.com/chinmina/kms-import/issues/3)) ([55db095](https://github.com/chinmina/kms-import/commit/55db095f6510bee014acf33b03ef06189b68836e))
* scaffold Phase 1 tracer-bullet build and tooling ([02a91aa](https://github.com/chinmina/kms-import/commit/02a91aa3205e3827a476d096a65db11391a45b62))
* signed, multi-platform releases with build-provenance attestations (Phase 9) ([#9](https://github.com/chinmina/kms-import/issues/9)) ([53b11f2](https://github.com/chinmina/kms-import/commit/53b11f25e8fa34cb26af0290386bbee624292ed8))


### Bug Fixes

* bound the in-memory lifetime of secrets handled by the import library ([#12](https://github.com/chinmina/kms-import/issues/12)) ([f905a3e](https://github.com/chinmina/kms-import/commit/f905a3ea92bc5781267624c5662299683a53ec45))
* create release tag explicitly in release-please flow ([#16](https://github.com/chinmina/kms-import/issues/16)) ([36393eb](https://github.com/chinmina/kms-import/commit/36393eb3b3884117b20a67006cb9c10aa26afcd8))
