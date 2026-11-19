# Captured prototype fixtures

These files are test-only and are not embedded or installed in easy-lima.

- `local.py`: pinned launcher used by ownership-refusal contracts; SHA-256 `55999dc949a226ad73cf3bd0084fa6eb4164162e9f296445c14f6b01e6d0333f`.
- `git-guest.py`: unmodified public-identity writer captured from the same local prototype on 2026-11-19; SHA-256 `6ee6e2a0ddf0d64f044c75efcb131a413f36705630476bf1c9c7c8868666ad1c`.

The opt-in live adoption test runs the latter only inside its disposable guest, using a generated unregistered test key's public half and example identity. It establishes the old settings before testing their preservation and retirement. Neither fixture receives the user's private credentials.
