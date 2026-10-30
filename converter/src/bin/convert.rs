// Vendored unchanged from erikastramecki/dregg-sui, groth16-gen/src/bin/convert.rs.
// General for any vanilla BN254 Groth16 proof: nothing here is circuit-specific.

//! gnark → Sui converter (the bridge). Reads gnark's exported BN254 point
//! coordinates (proof-export.json), rebuilds arkworks BN254 objects, VERIFIES with
//! arkworks (the correctness oracle: if this passes, the gnark→arkworks mapping is
//! right), then `serialize_compressed` into the EXACT bytes `sui::groth16` consumes.
//!
//!   cargo run --bin convert -- <path-to-proof-export.json>

use ark_bn254::{Bn254, Fq, Fq2, Fr, G1Affine, G2Affine};
use ark_ff::PrimeField;
use ark_groth16::{Groth16, Proof, VerifyingKey};
use ark_serialize::CanonicalSerialize;
use ark_snark::SNARK;
use serde_json::Value;

fn fq(h: &str) -> Fq {
    Fq::from_be_bytes_mod_order(&hex::decode(h).expect("hex"))
}
fn g1(v: &Value) -> G1Affine {
    G1Affine::new_unchecked(fq(v["x"].as_str().unwrap()), fq(v["y"].as_str().unwrap()))
}
// gnark exports E2 as {A0, A1}; arkworks Fq2 = c0 + c1·u. Map A0→c0, A1→c1.
fn g2(v: &Value) -> G2Affine {
    let x = Fq2::new(fq(v["x0"].as_str().unwrap()), fq(v["x1"].as_str().unwrap()));
    let y = Fq2::new(fq(v["y0"].as_str().unwrap()), fq(v["y1"].as_str().unwrap()));
    G2Affine::new_unchecked(x, y)
}

fn main() {
    let path = std::env::args().nth(1).unwrap_or_else(|| "proof-export.json".into());
    let j: Value = serde_json::from_reader(std::fs::File::open(&path).expect("open json")).unwrap();

    let vk = VerifyingKey::<Bn254> {
        alpha_g1: g1(&j["vk"]["alpha"]),
        beta_g2: g2(&j["vk"]["beta"]),
        gamma_g2: g2(&j["vk"]["gamma"]),
        delta_g2: g2(&j["vk"]["delta"]),
        gamma_abc_g1: j["vk"]["ic"].as_array().unwrap().iter().map(g1).collect(),
    };
    let proof = Proof::<Bn254> {
        a: g1(&j["proof"]["a"]),
        b: g2(&j["proof"]["b"]),
        c: g1(&j["proof"]["c"]),
    };
    let pub_inputs: Vec<Fr> = j["public"]
        .as_array()
        .unwrap()
        .iter()
        .map(|h| Fr::from_be_bytes_mod_order(&hex::decode(h.as_str().unwrap()).unwrap()))
        .collect();

    // THE ORACLE: verify the reconstructed arkworks objects. If this is false, the
    // point mapping (G2 ordering / negation) is wrong and Sui would also reject.
    let pvk = Groth16::<Bn254>::process_vk(&vk).expect("process_vk");
    let ok = Groth16::<Bn254>::verify_with_processed_vk(&pvk, &pub_inputs, &proof).expect("verify");
    eprintln!("arkworks verify of the gnark proof = {ok}");
    if !ok {
        eprintln!("MAPPING WRONG — adjust G2 Fq2 ordering or B/beta negation.");
        std::process::exit(1);
    }

    // Serialize EXACTLY as Sui expects (arkworks compressed canonical).
    let mut vkb = Vec::new();
    vk.serialize_compressed(&mut vkb).unwrap();
    let mut pb = Vec::new();
    proof.serialize_compressed(&mut pb).unwrap();
    let mut pib = Vec::new();
    for p in &pub_inputs {
        p.serialize_compressed(&mut pib).unwrap();
    }
    println!("VK_HEX={}", hex::encode(&vkb));
    println!("PROOF_HEX={}", hex::encode(&pb));
    println!("PUBLIC_HEX={}", hex::encode(&pib));
    eprintln!("VK_LEN={} PROOF_LEN={} PUB_LEN={}", vkb.len(), pb.len(), pib.len());
}
