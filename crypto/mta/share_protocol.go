// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package mta

import (
	"errors"
	"math/big"

	"github.com/ordinox/thorchain-tss-lib/common"
	"github.com/ordinox/thorchain-tss-lib/crypto"
	"github.com/ordinox/thorchain-tss-lib/crypto/paillier"
	"github.com/ordinox/thorchain-tss-lib/tss"
)

func AliceInit(
	pkA *paillier.PublicKey,
	a, cA, rA, NTildeB, h1B, h2B *big.Int,
) (pf *RangeProofAlice, err error) {
	return ProveRangeAlice(pkA, cA, NTildeB, h1B, h2B, a, rA)
}

func BobMid(
	pkA *paillier.PublicKey,
	pf *RangeProofAlice,
	b, cA, NTildeA, h1A, h2A, NTildeB, h1B, h2B *big.Int,
) (beta, cB, betaPrm *big.Int, piB *ProofBob, err error) {
	if !pf.Verify(pkA, NTildeB, h1B, h2B, cA) {
		err = errors.New("RangeProofAlice.Verify() returned false")
		return
	}
	q := tss.EC().Params().N
	q5 := new(big.Int).Mul(q, q)  // q^2
	q5 = new(big.Int).Mul(q5, q5) // q^4
	q5 = new(big.Int).Mul(q5, q)  // q^5
	betaPrm = common.GetRandomPositiveInt(q5)
	cBetaPrm, cRand, err := pkA.EncryptAndReturnRandomness(betaPrm)
	if err != nil {
		return
	}
	if cB, err = pkA.HomoMult(b, cA); err != nil {
		return
	}
	if cB, err = pkA.HomoAdd(cB, cBetaPrm); err != nil {
		return
	}
	beta = common.ModInt(q).Sub(zero, betaPrm)
	piB, err = ProveBob(pkA, NTildeA, h1A, h2A, cA, cB, b, betaPrm, cRand)
	return
}

func BobMidWC(
	pkA *paillier.PublicKey,
	pf *RangeProofAlice,
	b, cA, NTildeA, h1A, h2A, NTildeB, h1B, h2B *big.Int,
	B *crypto.ECPoint,
) (betaPrm, cB *big.Int, piB *ProofBobWC, err error) {
	if !pf.Verify(pkA, NTildeB, h1B, h2B, cA) {
		err = errors.New("RangeProofAlice.Verify() returned false")
		return
	}
	q := tss.EC().Params().N
	q5 := new(big.Int).Mul(q, q)  // q^2
	q5 = new(big.Int).Mul(q5, q5) // q^4
	q5 = new(big.Int).Mul(q5, q)  // q^5
	betaPrm = common.GetRandomPositiveInt(q5)
	cBetaPrm, cRand, err := pkA.EncryptAndReturnRandomness(betaPrm)
	if err != nil {
		return
	}
	cB, err = pkA.HomoMult(b, cA)
	if err != nil {
		return
	}
	cB, err = pkA.HomoAdd(cB, cBetaPrm)
	if err != nil {
		return
	}
	piB, err = ProveBobWC(pkA, NTildeA, h1A, h2A, cA, cB, b, betaPrm, cRand, B)
	return
}

func AliceEnd(
	pkA *paillier.PublicKey,
	pf *ProofBob,
	h1A, h2A, cA, cB, NTildeA *big.Int,
	sk *paillier.PrivateKey,
) (alphaIJ *big.Int, err error) {
	if !pf.Verify(pkA, NTildeA, h1A, h2A, cA, cB) {
		err = errors.New("ProofBob.Verify() returned false")
		return
	}
	if alphaIJ, err = sk.Decrypt(cB); err != nil {
		return
	}
	q := tss.EC().Params().N
	alphaIJ.Mod(alphaIJ, q)
	return
}

func AliceEndWC(
	pkA *paillier.PublicKey,
	pf *ProofBobWC,
	B *crypto.ECPoint,
	cA, cB, NTildeA, h1A, h2A *big.Int,
	sk *paillier.PrivateKey,
) (muIJ, muIJRec, muIJRand *big.Int, err error) {
	if !pf.Verify(pkA, NTildeA, h1A, h2A, cA, cB, B) {
		err = errors.New("ProofBobWC.Verify() returned false")
		return
	}
	if muIJRec, muIJRand, err = sk.DecryptAndRecoverRandomness(cB); err != nil {
		return
	}
	q := tss.EC().Params().N
	muIJ = new(big.Int).Mod(muIJRec, q)
	return
}
