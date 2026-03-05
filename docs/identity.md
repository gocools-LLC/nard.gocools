# Node Identity and Key Rotation

Implementation: `internal/security/identity`

## Identity Document Format

Document version: `nard.identity/v1`

Fields:

- `version`
- `node_id`
- `key_id`
- `algorithm` (`ed25519`)
- `public_key` (base64)
- `issued_at`
- `expires_at`
- `previous_key_id` (optional)
- `status` (`active`, `rotated`, `revoked`)
- `signature` (base64, Ed25519 over canonical document payload)

## Key Lifecycle Policy

- key pairs are generated per node identity document
- each document has `issued_at` and `expires_at`
- default rotation window: `24h` (configurable)
- rotation creates a new `key_id` and links to prior key via `previous_key_id`
- previous key is marked `rotated` and retained in history

## Trust Bootstrap Model

- initial trust is bootstrapped out-of-band using known node identity documents
- each node verifies:
  - document version and algorithm
  - signature validity
  - expiry window
  - revocation status
- key lineage is auditable through `previous_key_id` links

## Revocation and Compromised-Key Response

If a key is compromised:

1. mark current key as `revoked`
2. publish revocation record (`node_id`, `key_id`, `reason`, `revoked_at`)
3. immediately mint a replacement key document linked to the revoked `key_id`
4. reject future trust decisions for the revoked key

`HandleCompromise(reason)` implements this sequence.

## Verification APIs

- `VerifyDocument(document, asOf)`
- `VerifyDocumentWithRevocations(document, asOf, revocations)`

These APIs enforce signature checks and revocation/expiry policy.
