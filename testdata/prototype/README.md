# Pinned adoption fixture

`v1/local.py` is the exact original workspace launcher used to establish the
supported handoff contract (SHA-256
`55999dc949a226ad73cf3bd0084fa6eb4164162e9f296445c14f6b01e6d0333f`).
It is retained as test input, not installed or exposed as a compatibility command.
Do not run it against real projects or update its bytes when changing easy-lima.

The adoption tests construct disposable metadata around it. They check that the
old launcher's ownership gate rejects an instance after its active record is
retired. Current application code does not import or execute this fixture.
