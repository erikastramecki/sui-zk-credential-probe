#[test_only]
module dregg_verifier::cred_poseidon_match {
    use sui::poseidon::poseidon_bn254;

    const SECRET: u256 = 1234567;
    const NULL_SECRET: u256 = 7654321;
    const LEAF: u256 = 3625476295524753380583158575965417585927393704606287846937854484811148355651;
    const ROOT: u256 = 2635160645901770692864156253893997684210531022917187681615693762998018704563;
    const NULLIFIER: u256 = 19243209251525058199409480487736750184462717395322983903846384942246396038207;

    /// The circuit-side iden3 Poseidon gadget and Sui's native poseidon_bn254 must agree
    /// on EVERY node of the credential: the leaf, the 16-hop climb, and the nullifier.
    #[test]
    fun onchain_poseidon_reproduces_the_whole_credential() {
        let leaf = poseidon_bn254(&vector[SECRET, NULL_SECRET]);
        assert!(leaf == LEAF, 1);

        let mut cur = leaf;
        let mut i = 0u64;
        while (i < 16) {
            let sib = (1000 + i as u256);
            cur = if (i % 2 == 0) { poseidon_bn254(&vector[cur, sib]) }
                  else { poseidon_bn254(&vector[sib, cur]) };
            i = i + 1;
        };
        assert!(cur == ROOT, 2);

        assert!(poseidon_bn254(&vector[NULL_SECRET, leaf]) == NULLIFIER, 3);
    }
}
