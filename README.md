# sui-zk-credential-probe

A working, measured answer to one question: **can you prove a small Poseidon
membership statement off-chain and have Sui verify it natively, today?**

Yes. `bash run-all.sh` proves a depth-16 Poseidon Merkle membership credential
with a nullifier, converts it to Sui's byte format, and verifies that proof in
the Sui Move VM. It took **21 seconds** on an M-series Mac, start to finish.

This is a spike, not a product. Read the constraints before building on it.

## What it does

A credential circuit (`circuit/main.go`) proves, in zero knowledge:

- the prover knows a `secret` and a `nullSecret` whose Poseidon commitment
  `leaf = Poseidon(secret, nullSecret)` sits in a Merkle tree under a public `root`;
- the matching `nullifier = Poseidon(nullSecret, leaf)` — also public, so a verifier
  can spend-check it without learning which leaf was used.

Two public inputs: `(root, nullifier)`. Everything else is witness.

## Measured, not estimated

Every number below came out of `run-all.sh` on 2026-10-29. Re-run it; it prints them.

| | |
|---|---|
| Constraints, depth 16 | **4,370** (~270 per Poseidon hop) |
| gnark commitments | **0** — vanilla Groth16, which is all Sui verifies |
| Public inputs | 2, against a hard cap of 8 |
| Compile | 13 ms |
| Trusted setup | 276 ms |
| Prove | 42 ms |
| Convert + arkworks re-verify | ~350 ms |
| Sui Move VM verify | within a 3 s package test run |

At ~270 constraints per hop, a 20k-constraint budget buys roughly depth 48, or
depth 16 with a lot of other logic alongside it.

## Run it

Needs `go`, `cargo` and `sui` on PATH. No chain, no keys, no funds, no network
beyond fetching dependencies.

```bash
bash run-all.sh
```

Four steps: prove → convert → **regenerate the Move fixtures from that proof** →
verify. Step 3 matters: the fixtures in `move/dregg_verifier/tests/cred_probe.move`
are rewritten by `tools/write-fixtures.py` from the proof step 1 just made, so a
green test run is a statement about this run and not about a stale committed proof.

## The three things that will bite you

**1. Sui caps public inputs at 8.** Not a circuit choice — it is in the framework:

```move
// sui-framework/sources/crypto/groth16.move
const ETooManyPublicInputs: u64 = 2;
const MaxPublicInputs: u64 = 8;
```

It aborts in `prepare_verifying_key`, before the proof is examined.
`tests/pubcount_probe.move` is the executable form: 8 verifies, 9 aborts.
Fold anything wider into one field element inside the circuit.

**2. Poseidon parameters must match exactly, and a mismatch is silent.** The
circuit uses iden3/circomlib Poseidon with `initState = 0`; Sui's native
`sui::poseidon::poseidon_bn254` must agree bit for bit or everything built on top
is quietly broken. That agreement is pinned by an executable test, not a comment:
`tests/cred_poseidon_match.move` recomputes the leaf, all 16 hops **and** the
nullifier with Sui's own hash and asserts each against what the circuit proved.
Run it before you trust the gadget.

**3. The setup in this repo is a single-party development setup.** `circuit/main.go`
calls `groth16.Setup` in process, so one party transiently holds the trapdoor and
could forge a proof for any public input under that verifying key. A fresh verifying
key is generated on every run. **This cannot hold value.** Before anything real:
run a proper multi-party ceremony per circuit, or move to PLONK, whose universal
SRS removes the per-circuit ceremony entirely.

## What to reuse, and what to replace

Reuse unchanged — each of these was exercised by the run above:

- `converter/src/bin/convert.rs` — gnark → Sui bytes. General for any vanilla BN254
  Groth16 proof with any number of public inputs; nothing in it is circuit-specific.
  Its arkworks re-verify is the same check `sui::groth16` runs, so a bad point mapping
  fails locally instead of on chain.
- `move/dregg_verifier/sources/verifier.move` — the native verify. Took the credential
  proof as-is.
- `circuit/poseidon_gadget.go` + `circuit/iden3_constants.go` — the in-circuit Poseidon.

Replace:

- **`circuit/main.go`** is a skeleton. It has no epoch or registry binding, no range
  check on the leaf index, and the nullifier is not domain-separated. Design yours.
- **`move/dregg_verifier/sources/settlement.move`** is included as reference only and
  should not gate a credential as written. Three gaps: the verifying key is supplied by
  whoever calls `fund` rather than pinned by the module; there is no nullifier/spent set,
  so a valid proof replays; and there is no refund or timeout, so funds lock permanently
  if no valid proof ever exists.

## Provenance

`converter/` and `move/dregg_verifier/sources/` are vendored unchanged from the
author's own [erikastramecki/dregg-sui](https://github.com/erikastramecki/dregg-sui),
a Sui settlement backend for dregg proofs. The Poseidon gadget and its circomlib
constant tables are reused from the author's own circuit work. Everything under
`circuit/main.go`, `tools/`, `run-all.sh` and the three `tests/*probe*.move` /
`cred_*.move` files is new here.

Toolchain pinned by the lockfiles: gnark v0.11.0, gnark-crypto v0.14.0,
go-iden3-crypto v0.0.17, arkworks 0.4, Sui CLI 1.75.1.
