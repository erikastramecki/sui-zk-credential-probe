// Anonymous-bidder credential probe: Poseidon membership in a depth-D tree + a
// nullifier, both public. Reuses assay's iden3 Poseidon gadget (bit-exact with
// sui::poseidon_bn254) and the export shape from dregg-sui gnark-gen/vanilla.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"

	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark-crypto/ecc"
	iden3 "github.com/iden3/go-iden3-crypto/poseidon"
)

const Depth = 16

type CredCircuit struct {
	Secret     frontend.Variable        `gnark:",secret"`
	NullSecret frontend.Variable        `gnark:",secret"`
	PathBits   [Depth]frontend.Variable `gnark:",secret"`
	Siblings   [Depth]frontend.Variable `gnark:",secret"`
	Root       frontend.Variable        `gnark:",public"`
	Nullifier  frontend.Variable        `gnark:",public"`
}

func (c *CredCircuit) Define(api frontend.API) error {
	leaf := PoseidonBn254(api, []frontend.Variable{c.Secret, c.NullSecret})
	cur := leaf
	for i := 0; i < Depth; i++ {
		api.AssertIsBoolean(c.PathBits[i])
		left := api.Select(c.PathBits[i], c.Siblings[i], cur)
		right := api.Select(c.PathBits[i], cur, c.Siblings[i])
		cur = PoseidonBn254(api, []frontend.Variable{left, right})
	}
	api.AssertIsEqual(cur, c.Root)
	api.AssertIsEqual(PoseidonBn254(api, []frontend.Variable{c.NullSecret, leaf}), c.Nullifier)
	return nil
}

func h2(a, b *big.Int) *big.Int {
	o, e := iden3.Hash([]*big.Int{a, b})
	if e != nil { panic(e) }
	return o
}
func fpHex(e fp.Element) string { b := e.Bytes(); return hex.EncodeToString(b[:]) }
func g1(p curve.G1Affine) map[string]string { return map[string]string{"x": fpHex(p.X), "y": fpHex(p.Y)} }
func g2(p curve.G2Affine) map[string]string {
	return map[string]string{"x0": fpHex(p.X.A0), "x1": fpHex(p.X.A1), "y0": fpHex(p.Y.A0), "y1": fpHex(p.Y.A1)}
}

func main() {
	secret := big.NewInt(1234567)
	nullSecret := big.NewInt(7654321)
	leaf := h2(secret, nullSecret)

	bits := make([]int64, Depth)
	sibs := make([]*big.Int, Depth)
	cur := new(big.Int).Set(leaf)
	for i := 0; i < Depth; i++ {
		bits[i] = int64(i % 2)
		sibs[i] = big.NewInt(int64(1000 + i))
		if bits[i] == 0 { cur = h2(cur, sibs[i]) } else { cur = h2(sibs[i], cur) }
	}
	root := cur
	null := h2(nullSecret, leaf)

	t0 := time.Now()
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &CredCircuit{})
	if err != nil { panic(err) }
	tCompile := time.Since(t0)
	fmt.Fprintf(os.Stderr, "depth=%d constraints=%d  compile=%s\n", Depth, ccs.GetNbConstraints(), tCompile.Round(time.Millisecond))

	t1 := time.Now()
	pk, vk, err := groth16.Setup(ccs)
	if err != nil { panic(err) }
	tSetup := time.Since(t1)

	var a CredCircuit
	a.Secret, a.NullSecret, a.Root, a.Nullifier = secret, nullSecret, root, null
	for i := 0; i < Depth; i++ { a.PathBits[i] = bits[i]; a.Siblings[i] = sibs[i] }
	w, err := frontend.NewWitness(&a, ecc.BN254.ScalarField())
	if err != nil { panic(err) }

	t2 := time.Now()
	proof, err := groth16.Prove(ccs, pk, w)
	if err != nil { panic(err) }
	tProve := time.Since(t2)

	pub, _ := w.Public()
	if err := groth16.Verify(proof, vk, pub); err != nil { panic(err) }
	nComm := len(proof.(*groth16bn254.Proof).Commitments)
	fmt.Fprintf(os.Stderr, "setup=%s prove=%s commitments=%d publics=2\n",
		tSetup.Round(time.Millisecond), tProve.Round(time.Millisecond), nComm)

	p := proof.(*groth16bn254.Proof); v := vk.(*groth16bn254.VerifyingKey)
	ic := make([]map[string]string, len(v.G1.K))
	for i := range v.G1.K { ic[i] = g1(v.G1.K[i]) }
	var rf, nf fr.Element
	rf.SetBigInt(root); nf.SetBigInt(null)
	rb := rf.Bytes(); nb := nf.Bytes()
	out := map[string]interface{}{
		"proof":  map[string]interface{}{"a": g1(p.Ar), "b": g2(p.Bs), "c": g1(p.Krs)},
		"vk":     map[string]interface{}{"alpha": g1(v.G1.Alpha), "beta": g2(v.G2.Beta), "gamma": g2(v.G2.Gamma), "delta": g2(v.G2.Delta), "ic": ic},
		"public": []string{hex.EncodeToString(rb[:]), hex.EncodeToString(nb[:])},
	}
	f, _ := os.Create("proof-export.json"); defer f.Close()
	enc := json.NewEncoder(f); enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil { panic(err) }
	fmt.Fprintln(os.Stderr, "wrote proof-export.json (root, nullifier)")
	fmt.Fprintf(os.Stderr, "root=%s\nnullifier=%s\n", root.String(), null.String())
}
