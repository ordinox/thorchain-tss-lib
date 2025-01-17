// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package mta

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ordinox/thorchain-tss-lib/common"
	"github.com/ordinox/thorchain-tss-lib/crypto"
	"github.com/ordinox/thorchain-tss-lib/crypto/paillier"
	"github.com/ordinox/thorchain-tss-lib/tss"
)

const (
	ProofBobBytesParts   = 10
	ProofBobWCBytesParts = 12
)

type (
	ProofBob struct {
		Z, ZPrm, T, V, W, S, S1, S2, T1, T2 *big.Int
	}

	ProofBobWC struct {
		*ProofBob
		U *crypto.ECPoint
	}
)

// ProveBobWC implements Bob's proof both with or without check "ProveMtawc_Bob" and "ProveMta_Bob" used in the MtA protocol from GG18Spec (9) Figs. 10 & 11.
// an absent `X` generates the proof without the X consistency check X = g^x
func ProveBobWC(pk *paillier.PublicKey, NTilde, h1, h2, c1, c2, x, y, r *big.Int, X *crypto.ECPoint) (*ProofBobWC, error) {
	if pk == nil || NTilde == nil || h1 == nil || h2 == nil || c1 == nil || c2 == nil || x == nil || y == nil || r == nil {
		return nil, errors.New("ProveBob() received a nil argument")
	}

	NSq := pk.NSquare()

	q := tss.EC().Params().N
	q2 := new(big.Int).Mul(q, q)
	q3 := new(big.Int).Mul(q2, q)
	q6 := new(big.Int).Mul(q3, q3)
	q7 := new(big.Int).Mul(q6, q)
	qNTilde := new(big.Int).Mul(q, NTilde)
	q3NTilde := new(big.Int).Mul(q3, NTilde)

	// steps are numbered as shown in Fig. 10, but diverge slightly for Fig. 11
	// 1.
	alpha := common.GetRandomPositiveInt(q3)

	// 2.
	rho := common.GetRandomPositiveInt(qNTilde)
	sigma := common.GetRandomPositiveInt(qNTilde)
	tau := common.GetRandomPositiveInt(q3NTilde)

	// 3.
	rhoPrm := common.GetRandomPositiveInt(q3NTilde)

	// 4.
	beta := common.GetRandomPositiveRelativelyPrimeInt(pk.N)
	gamma := common.GetRandomPositiveInt(q7)

	// 5.
	u := crypto.NewECPointNoCurveCheck(tss.EC(), zero, zero) // initialization suppresses an IDE warning
	if X != nil {
		u = crypto.ScalarBaseMult(tss.EC(), alpha)
	}

	// 6.
	modNTilde := common.ModInt(NTilde)
	z := modNTilde.Exp(h1, x)
	z = modNTilde.Mul(z, modNTilde.Exp(h2, rho))

	// 7.
	zPrm := modNTilde.Exp(h1, alpha)
	zPrm = modNTilde.Mul(zPrm, modNTilde.Exp(h2, rhoPrm))

	// 8.
	t := modNTilde.Exp(h1, y)
	t = modNTilde.Mul(t, modNTilde.Exp(h2, sigma))

	// 9.
	modNSq := common.ModInt(NSq)
	v := modNSq.Exp(c1, alpha)
	v = modNSq.Mul(v, modNSq.Exp(pk.Gamma(), gamma))
	v = modNSq.Mul(v, modNSq.Exp(beta, pk.N))

	// 10.
	w := modNTilde.Exp(h1, gamma)
	w = modNTilde.Mul(w, modNTilde.Exp(h2, tau))

	// 11-12. e'
	var e *big.Int
	{ // must use RejectionSample
		var eHash *big.Int
		// X is nil if called by ProveBob (Bob's proof "without check")
		if X == nil {
			eHash = common.SHA512_256i(append(pk.AsInts(), c1, c2, z, zPrm, t, v, w)...)
		} else {
			eHash = common.SHA512_256i(append(pk.AsInts(), X.X(), X.Y(), c1, c2, u.X(), u.Y(), z, zPrm, t, v, w)...)
		}
		e = common.RejectionSample(q, eHash)
	}

	// 13.
	modN := common.ModInt(pk.N)
	s := modN.Exp(r, e)
	s = modN.Mul(s, beta)

	// 14.
	s1 := new(big.Int).Mul(e, x)
	s1 = s1.Add(s1, alpha)

	// 15.
	s2 := new(big.Int).Mul(e, rho)
	s2 = s2.Add(s2, rhoPrm)

	// 16.
	t1 := new(big.Int).Mul(e, y)
	t1 = t1.Add(t1, gamma)

	// 17.
	t2 := new(big.Int).Mul(e, sigma)
	t2 = t2.Add(t2, tau)

	// the regular Bob proof ("without check") is extracted and returned by ProveBob
	pf := &ProofBob{Z: z, ZPrm: zPrm, T: t, V: v, W: w, S: s, S1: s1, S2: s2, T1: t1, T2: t2}

	// or the WC ("with check") version is used in round 2 of the signing protocol
	return &ProofBobWC{ProofBob: pf, U: u}, nil
}

// ProveBob implements Bob's proof "ProveMta_Bob" used in the MtA protocol from GG18Spec (9) Fig. 11.
func ProveBob(pk *paillier.PublicKey, NTilde, h1, h2, c1, c2, x, y, r *big.Int) (*ProofBob, error) {
	// the Bob proof ("with check") contains the ProofBob "without check"; this method extracts and returns it
	// X is supplied as nil to exclude it from the proof hash
	pf, err := ProveBobWC(pk, NTilde, h1, h2, c1, c2, x, y, r, nil)
	if err != nil {
		return nil, err
	}
	return pf.ProofBob, nil
}

func ProofBobWCFromBytes(bzs [][]byte) (*ProofBobWC, error) {
	proofBob, err := ProofBobFromBytes(bzs)
	if err != nil {
		return nil, err
	}
	point, err := crypto.NewECPoint(tss.EC(),
		new(big.Int).SetBytes(bzs[10]),
		new(big.Int).SetBytes(bzs[11]))
	if err != nil {
		return nil, err
	}
	return &ProofBobWC{
		ProofBob: proofBob,
		U:        point,
	}, nil
}

func ProofBobFromBytes(bzs [][]byte) (*ProofBob, error) {
	if !common.NonEmptyMultiBytes(bzs, ProofBobBytesParts) &&
		!common.NonEmptyMultiBytes(bzs, ProofBobWCBytesParts) {
		return nil, fmt.Errorf(
			"expected %d byte parts to construct ProofBob, or %d for ProofBobWC",
			ProofBobBytesParts, ProofBobWCBytesParts)
	}
	return &ProofBob{
		Z:    new(big.Int).SetBytes(bzs[0]),
		ZPrm: new(big.Int).SetBytes(bzs[1]),
		T:    new(big.Int).SetBytes(bzs[2]),
		V:    new(big.Int).SetBytes(bzs[3]),
		W:    new(big.Int).SetBytes(bzs[4]),
		S:    new(big.Int).SetBytes(bzs[5]),
		S1:   new(big.Int).SetBytes(bzs[6]),
		S2:   new(big.Int).SetBytes(bzs[7]),
		T1:   new(big.Int).SetBytes(bzs[8]),
		T2:   new(big.Int).SetBytes(bzs[9]),
	}, nil
}

// ProveBobWC.Verify implements verification of Bob's proof with check "VerifyMtawc_Bob" used in the MtA protocol from GG18Spec (9) Fig. 10.
// an absent `X` verifies a proof generated without the X consistency check X = g^x
func (pf *ProofBobWC) Verify(pk *paillier.PublicKey, NTilde, h1, h2, c1, c2 *big.Int, X *crypto.ECPoint) bool {
	if pk == nil || NTilde == nil || h1 == nil || h2 == nil || c1 == nil || c2 == nil {
		return false
	}

	q := tss.EC().Params().N
	q2 := new(big.Int).Mul(q, q)
	q3 := new(big.Int).Mul(q, q2)
	q6 := new(big.Int).Mul(q3, q3) // q^6
	q7 := new(big.Int).Mul(q6, q)  // q^7

	if !common.IsInInterval(pf.Z, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.ZPrm, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.T, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.V, pk.NSquare()) {
		return false
	}
	if !common.IsInInterval(pf.W, NTilde) {
		return false
	}
	if !common.IsInInterval(pf.S, pk.N) {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.Z, NTilde).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.ZPrm, NTilde).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.T, NTilde).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.V, pk.NSquare()).Cmp(one) != 0 {
		return false
	}
	if new(big.Int).GCD(nil, nil, pf.W, NTilde).Cmp(one) != 0 {
		return false
	}

	gcd := big.NewInt(0)
	if pf.S.Cmp(zero) == 0 {
		return false
	}
	if gcd.GCD(nil, nil, pf.S, pk.N).Cmp(one) != 0 {
		return false
	}
	if pf.V.Cmp(zero) == 0 {
		return false
	}
	if gcd.GCD(nil, nil, pf.V, pk.N).Cmp(one) != 0 {
		return false
	}
	// 3.
	if pf.S1.Cmp(q3) > 0 {
		return false
	}
	if pf.T1.Cmp(q7) > 0 {
		return false
	}

	// 1-2. e'
	var e *big.Int
	{ // must use RejectionSample
		var eHash *big.Int
		// X is nil if called on a ProveBob (Bob's proof "without check")
		if X == nil {
			eHash = common.SHA512_256i(append(pk.AsInts(), c1, c2, pf.Z, pf.ZPrm, pf.T, pf.V, pf.W)...)
		} else {
			eHash = common.SHA512_256i(append(pk.AsInts(), X.X(), X.Y(), c1, c2, pf.U.X(), pf.U.Y(), pf.Z, pf.ZPrm, pf.T, pf.V, pf.W)...)
		}
		e = common.RejectionSample(q, eHash)
	}

	var left, right *big.Int // for the following conditionals

	// 4. runs only in the "with check" mode from Fig. 10
	if X != nil {
		s1ModQ := new(big.Int).Mod(pf.S1, tss.EC().Params().N)
		gS1 := crypto.ScalarBaseMult(tss.EC(), s1ModQ)
		xEU, err := X.ScalarMult(e).Add(pf.U)
		if err != nil || !gS1.Equals(xEU) {
			return false
		}
	}

	{ // 5-6.
		modNTilde := common.ModInt(NTilde)

		{ // 5.
			h1ExpS1 := modNTilde.Exp(h1, pf.S1)
			h2ExpS2 := modNTilde.Exp(h2, pf.S2)
			left = modNTilde.Mul(h1ExpS1, h2ExpS2)
			zExpE := modNTilde.Exp(pf.Z, e)
			right = modNTilde.Mul(zExpE, pf.ZPrm)
			if left.Cmp(right) != 0 {
				return false
			}
		}

		{ // 6.
			h1ExpT1 := modNTilde.Exp(h1, pf.T1)
			h2ExpT2 := modNTilde.Exp(h2, pf.T2)
			left = modNTilde.Mul(h1ExpT1, h2ExpT2)
			tExpE := modNTilde.Exp(pf.T, e)
			right = modNTilde.Mul(tExpE, pf.W)
			if left.Cmp(right) != 0 {
				return false
			}
		}
	}

	{ // 7.
		modNSq := common.ModInt(pk.NSquare())

		c1ExpS1 := modNSq.Exp(c1, pf.S1)
		sExpN := modNSq.Exp(pf.S, pk.N)
		gammaExpT1 := modNSq.Exp(pk.Gamma(), pf.T1)
		left = modNSq.Mul(c1ExpS1, sExpN)
		left = modNSq.Mul(left, gammaExpT1)
		c2ExpE := modNSq.Exp(c2, e)
		right = modNSq.Mul(c2ExpE, pf.V)
		if left.Cmp(right) != 0 {
			return false
		}
	}
	return true
}

// ProveBob.Verify implements verification of Bob's proof without check "VerifyMta_Bob" used in the MtA protocol from GG18Spec (9) Fig. 11.
func (pf *ProofBob) Verify(pk *paillier.PublicKey, NTilde, h1, h2, c1, c2 *big.Int) bool {
	if pf == nil {
		return false
	}
	pfWC := &ProofBobWC{ProofBob: pf, U: nil}
	return pfWC.Verify(pk, NTilde, h1, h2, c1, c2, nil)
}

func (pf *ProofBob) ValidateBasic() bool {
	return pf.Z != nil &&
		pf.ZPrm != nil &&
		pf.T != nil &&
		pf.V != nil &&
		pf.W != nil &&
		pf.S != nil &&
		pf.S1 != nil &&
		pf.S2 != nil &&
		pf.T1 != nil &&
		pf.T2 != nil
}

func (pf *ProofBobWC) ValidateBasic() bool {
	return pf.ProofBob.ValidateBasic() && pf.U != nil
}

func (pf *ProofBob) Bytes() [ProofBobBytesParts][]byte {
	return [...][]byte{
		pf.Z.Bytes(),
		pf.ZPrm.Bytes(),
		pf.T.Bytes(),
		pf.V.Bytes(),
		pf.W.Bytes(),
		pf.S.Bytes(),
		pf.S1.Bytes(),
		pf.S2.Bytes(),
		pf.T1.Bytes(),
		pf.T2.Bytes(),
	}
}

func (pf *ProofBobWC) Bytes() [ProofBobWCBytesParts][]byte {
	var out [ProofBobWCBytesParts][]byte
	bobBzs := pf.ProofBob.Bytes()
	bobBzsSlice := bobBzs[:]
	bobBzsSlice = append(bobBzsSlice, pf.U.X().Bytes())
	bobBzsSlice = append(bobBzsSlice, pf.U.Y().Bytes())
	copy(out[:], bobBzsSlice[:12])
	return out
}
