// SPDX-License-Identifier: BUSL-1.1
//
// Copyright (C) 2024, Berachain Foundation. All rights reserved.
// Use of this software is governed by the Business Source License included
// in the LICENSE file of this repository and at www.mariadb.com/bsl11.
//
// ANY USE OF THE LICENSED WORK IN VIOLATION OF THIS LICENSE WILL AUTOMATICALLY
// TERMINATE YOUR RIGHTS UNDER THIS LICENSE FOR THE CURRENT AND ALL OTHER
// VERSIONS OF THE LICENSED WORK.
//
// THIS LICENSE DOES NOT GRANT YOU ANY RIGHT IN ANY TRADEMARK OR LOGO OF
// LICENSOR OR ITS AFFILIATES (PROVIDED THAT YOU MAY USE A TRADEMARK OR LOGO OF
// LICENSOR AS EXPRESSLY REQUIRED BY THIS LICENSE).
//
// TO THE EXTENT PERMITTED BY APPLICABLE LAW, THE LICENSED WORK IS PROVIDED ON
// AN “AS IS” BASIS. LICENSOR HEREBY DISCLAIMS ALL WARRANTIES AND CONDITIONS,
// EXPRESS OR IMPLIED, INCLUDING (WITHOUT LIMITATION) WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, NON-INFRINGEMENT, AND
// TITLE.

package core

import (
	constypes "github.com/berachain/beacon-kit/consensus-types/types"
	"github.com/berachain/beacon-kit/primitives/common"
	"github.com/berachain/beacon-kit/primitives/crypto"
	"github.com/berachain/beacon-kit/primitives/version"
)

func (sp *StateProcessor[
	_, _, BeaconBlockHeaderT, BeaconStateT,
	_, _, _, _, _, _, ForkDataT, _, _, _, _, _, _,
]) GetSidecarVerifierFn(
	st BeaconStateT,
) (
	func(blkHeader BeaconBlockHeaderT, signature crypto.BLSSignature) error,
	error,
) {
	slot, err := st.GetSlot()
	if err != nil {
		return nil, err
	}
	epoch := sp.cs.SlotToEpoch(slot)

	genesisValidatorsRoot, err := st.GetGenesisValidatorsRoot()
	if err != nil {
		return nil, err
	}

	var fd ForkDataT
	fd = fd.New(
		version.FromUint32[common.Version](
			sp.cs.ActiveForkVersionForEpoch(epoch),
		), genesisValidatorsRoot,
	)
	//nolint:errcheck // safe
	domain := any(fd).(*constypes.ForkData).ComputeDomain(
		sp.cs.DomainTypeProposer(),
	)

	return func(
		blkHeader BeaconBlockHeaderT,
		signature crypto.BLSSignature,
	) error {
		//nolint:govet // shadow
		proposer, err := st.ValidatorByIndex(blkHeader.GetProposerIndex())
		if err != nil {
			return err
		}
		signingRoot := constypes.ComputeSigningRoot(
			blkHeader,
			domain,
		)
		return sp.signer.VerifySignature(
			proposer.GetPubkey(),
			signingRoot[:],
			signature,
		)
	}, nil
}
