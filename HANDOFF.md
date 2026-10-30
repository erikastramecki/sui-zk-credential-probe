# Handoff — pick this up on the VPS

Point a fresh session at this file. It is the whole brief: what is already proven,
what is still open, and what not to do. Written 2026-10-29.

## Start here, in this order

```bash
git clone https://github.com/erikastramecki/sui-zk-credential-probe.git
cd sui-zk-credential-probe
bash run-all.sh          # ~21s on a laptop; longer on first dependency fetch
cat README.md            # the measured numbers and the three gotchas
```

If `run-all.sh` goes green you have the full pipeline working locally and you can
trust every number in the README, because you just reproduced them. If it does not,
fix that before anything else — do not start designing on top of a broken loop.

Toolchain: `go` (1.23+), `cargo`, and the `sui` CLI. Nothing else. No chain access,
no keys, no funds are needed, and none should be configured.

## The goal this feeds

An **anonymous-bidder credential**: a bidder proves membership in a permitted set
without revealing which member they are, and emits a nullifier so the same credential
cannot be used twice. Roughly 10k–50k constraints was the expected budget.

The spike says that budget is comfortable. A depth-16 Poseidon membership proof plus
nullifier is **4,370 constraints** and proves in **42 ms**. The ceiling is not the
prover — it is the eight-public-input cap on Sui's native verifier and the trusted
setup, both covered in the README.

## What is settled, and how it was settled

Each of these was verified by running code on 2026-10-29, not by reading docs:

- A vanilla BN254 Groth16 proof with 2 public inputs verifies in the Sui Move VM.
- The converter is general — it handled a 2-input proof with no modification, and
  contains nothing circuit-specific.
- The circuit's iden3 Poseidon and Sui's native `sui::poseidon_bn254` agree bit for
  bit on the leaf, all 16 hops and the nullifier.
- Sui's public-input cap is 8, enforced in the framework, aborting with code 2 before
  the proof is read. 8 passes, 9 aborts.
- The trusted setup here is single-party and in-process. A fresh verifying key every
  run. Not value-bearing.

## What is open — the actual work

1. **Design the real circuit.** `circuit/main.go` is a skeleton: no epoch or registry
   binding, no range check on the leaf index, nullifier not domain-separated. Decide
   the statement, then build it.
2. **Write the Move side properly.** The bundled `settlement.move` is reference only
   (caller-supplied verifying key, no spent set, no refund path — see README). A
   credential needs: the verifying key pinned in the module or a shared registry
   object, a nullifier set that rejects replays, and a decision on what happens when
   no valid proof ever arrives.
3. **Decide Groth16-with-a-ceremony versus PLONK.** Groth16 needs a fresh multi-party
   ceremony for every circuit change, which is a security-sensitive event each time.
   PLONK's universal SRS removes that. This choice shapes everything downstream, so
   make it early rather than discovering it later.
4. **Set the tree depth** from the real permitted-set size, then re-measure. ~270
   constraints per hop is the number to plan with.

## Rules for this work

- **Read-only against anything live.** No pushes, no deploys, nothing outward-facing
  without explicit per-instance approval. Approval for one action never carries to the
  next one.
- **Do not touch mainnet, and do not configure keys.** Everything here runs locally in
  the Move VM. There is no reason to go near a chain to do the circuit work.
- **Reproduce any suspected bug at least three times** before calling it real or
  reporting it anywhere, and rule out your own setup first.
- **Trust code over documentation.** Where a comment and the code disagree here, the
  code is right. Every claim in the README is backed by something you can run.
- Cite `file:line` for factual claims, or mark them UNVERIFIED.

## Related repositories

- [erikastramecki/dregg-sui](https://github.com/erikastramecki/dregg-sui) — the Sui
  settlement backend for dregg proofs that `converter/` and the Move sources came from.
  Also carries `VANILLA-RECONSTRUCTION.md`, which documents a much larger FRI-verifying
  circuit. **That work is unrelated to the credential and is not needed here** — do not
  get pulled into it.
- The `dregg` fork and its proving pipeline need a large box and are likewise not
  required for any of the above.
