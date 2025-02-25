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

package deposit

import (
	"context"
	"fmt"
	"sync"

	sdkcollections "cosmossdk.io/collections"
	"cosmossdk.io/core/store"
	ctypes "github.com/berachain/beacon-kit/consensus-types/types"
	"github.com/berachain/beacon-kit/errors"
	"github.com/berachain/beacon-kit/log"
	"github.com/berachain/beacon-kit/storage/encoding"
	"github.com/berachain/beacon-kit/storage/pruner"
)

const KeyDepositPrefix = "deposit"

// KVStore is a simple KV store based implementation that assumes
// the deposit indexes are tracked outside of the kv store.
type KVStore struct {
	store sdkcollections.Map[uint64, *ctypes.Deposit]

	// mu protects store for concurrent access
	mu sync.RWMutex

	// logger is used for logging information and errors.
	logger log.Logger
}

// NewStore creates a new deposit store.
func NewStore(
	kvsp store.KVStoreService,
	logger log.Logger,
) *KVStore {
	schemaBuilder := sdkcollections.NewSchemaBuilder(kvsp)
	res := &KVStore{
		store: sdkcollections.NewMap(
			schemaBuilder,
			sdkcollections.NewPrefix([]byte(KeyDepositPrefix)),
			KeyDepositPrefix,
			sdkcollections.Uint64Key,
			encoding.SSZValueCodec[*ctypes.Deposit]{},
		),
		logger: logger,
	}
	if _, err := schemaBuilder.Build(); err != nil {
		panic(fmt.Errorf("failed building KVStore schema: %w", err))
	}
	return res
}

// GetDepositsByIndex returns the first N deposits starting from the given
// index. If N is greater than the number of deposits, it returns up to the
// last deposit.
func (kv *KVStore) GetDepositsByIndex(
	startIndex uint64,
	depRange uint64,
) ([]*ctypes.Deposit, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	var (
		deposits = []*ctypes.Deposit{}
		endIdx   = startIndex + depRange
	)

	kv.logger.Debug(
		"GetDepositsByIndex request",
		"start", startIndex,
		"end", endIdx,
	)
	for i := startIndex; i < endIdx; i++ {
		deposit, err := kv.store.Get(context.TODO(), i)
		switch {
		case err == nil:
			deposits = append(deposits, deposit)
		case errors.Is(err, sdkcollections.ErrNotFound):
			kv.logger.Debug(
				"GetDepositsByIndex response",
				"start", startIndex,
				"end", i,
			)
			return deposits, nil
		default:
			return deposits, errors.Wrapf(
				err,
				"failed to get deposit %d, start: %d, end: %d",
				i, startIndex, endIdx,
			)
		}
	}

	kv.logger.Debug(
		"GetDepositsByIndex response",
		"start", startIndex,
		"end", endIdx,
	)
	return deposits, nil
}

// EnqueueDeposits pushes multiple deposits to the queue.
func (kv *KVStore) EnqueueDeposits(deposits []*ctypes.Deposit) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.logger.Debug(
		"EnqueueDeposits request",
		"to enqueue", len(deposits),
	)
	for _, deposit := range deposits {
		idx := deposit.GetIndex().Unwrap()
		kv.logger.Debug(
			"EnqueueDeposit response",
			"index", idx,
		)
		if err := kv.store.Set(
			context.TODO(),
			idx,
			deposit,
		); err != nil {
			return errors.Wrapf(err, "failed to enqueue deposit %d", idx)
		}
	}

	kv.logger.Debug(
		"EnqueueDeposit response",
		"enqueued", len(deposits),
	)
	return nil
}

// Prune removes the [start, end) deposits from the store.
func (kv *KVStore) Prune(start, end uint64) error {
	kv.logger.Debug(
		"Prune request",
		"start", start,
		"end", end,
	)
	if start > end {
		return fmt.Errorf(
			"DepositKVStore Prune start: %d, end: %d: %w",
			start, end, pruner.ErrInvalidRange,
		)
	}

	var ctx = context.TODO()
	kv.mu.Lock()
	defer kv.mu.Unlock()
	for i := range end {
		// This only errors if the key passed in cannot be encoded.
		if err := kv.store.Remove(ctx, start+i); err != nil {
			return errors.Wrapf(err, "failed to prune deposit %d", start+i)
		}
	}

	kv.logger.Debug(
		"Prune response",
		"start", start,
		"end", end,
	)
	return nil
}
