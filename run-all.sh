#!/usr/bin/env bash
# Reproduce the whole loop end to end: prove a Poseidon membership credential,
# convert it to Sui's byte format, regenerate the Move fixtures from THAT proof,
# and verify it in the Sui Move VM. No chain, no keys, no funds.
#
#   bash run-all.sh
#
# Needs: go, cargo, sui. Under a minute on a laptop after the first dependency fetch.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"

echo "== 1/4  prove (gnark, BN254 Groth16, fresh single-party dev setup) =="
( cd "$HERE/circuit" && GOFLAGS=-mod=mod go run . )

echo
echo "== 2/4  convert gnark -> Sui bytes (the arkworks re-verify is the oracle) =="
( cd "$HERE/converter" && cargo run -q --bin convert -- ../circuit/proof-export.json ) \
  | python3 "$HERE/tools/write-fixtures.py"

echo
echo "== 3/4  verify THAT proof in the Sui Move VM =="
( cd "$HERE/move/dregg_verifier" && sui move test --build-env testnet 2>&1 | sed -n '/Running Move unit tests/,$p' )

echo
echo "== 4/4  done =="
echo "The proof step 1 produced is the proof step 3 verified, and sui::poseidon"
echo "independently recomputed the leaf, the 16-hop climb and the nullifier."
