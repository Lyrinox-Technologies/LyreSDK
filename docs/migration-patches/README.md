# Legacy working copies

These two older working copies have no Git repository in the workspace:
`Projects/Libraries/liblyre` and `Projects/Server/lyre-test-service`.
Their source/module migration was applied locally and tests passed. These patches
preserve that work in Git for anyone reconstructing those copies. They upgrade
to canonical RDGProto 1.0.3 and, for the test service, the current LyreSDK.
Inspect and use `git apply --check` against the matching old source first.
Prefer LyreSDK for new development. No binaries or credentials are in the patches.
