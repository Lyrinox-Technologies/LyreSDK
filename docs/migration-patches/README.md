# Legacy working copies

These older working copies have no Git repository in the workspace:
`Projects/Libraries/liblyre`, `Projects/Server/lyre-test-service`, and
`Projects/Lyre-Services/lyrinox-lyre-provider`.
Their source/module migration was applied locally and tests passed. These patches
preserve that work in Git for anyone reconstructing those copies. They upgrade
to canonical RDGProto 1.0.3 and, for the test service, the current LyreSDK.
Inspect and use `git apply --check` against the matching old source first.
Prefer LyreSDK for new development. No binaries or credentials are in the patches.

The official provider patch also updates the integration test to provider-pinned
capability calls through the current SDK. Its tests passed before deployment.
