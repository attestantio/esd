// Copyright © 2021, 2024 Attestant Limited.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package head

import (
	"context"
	"fmt"
	"sort"

	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/attestantio/go-eth2-client/api"
	spec "github.com/attestantio/go-eth2-client/spec/phase0"
)

// OnHeadUpdated handles head notifications.
func (s *Service) OnHeadUpdated(
	ctx context.Context,
	_ spec.Slot,
	blockRoot spec.Root,
) {
	// Fetch the block.
	blockResponse, err := s.eth2Client.(eth2client.SignedBeaconBlockProvider).SignedBeaconBlock(ctx, &api.SignedBeaconBlockOpts{
		Block: blockRoot.String(),
	})
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to obtain block")
		return
	}
	block := blockResponse.Data
	s.log.Trace().Str("block_root", fmt.Sprintf("%#x", blockRoot)).Msg("Obtained block")

	attesterSlashings, err := block.AttesterSlashings()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to obtain attester slashings")
	}
	proposerSlashings, err := block.ProposerSlashings()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to obtain proposer slashings")
	}

	if len(attesterSlashings) == 0 &&
		len(proposerSlashings) == 0 {
		s.log.Trace().Msg("No slashings")
		return
	}

	// Report on attester slashings.
	s.attesterSlashings(ctx, attesterSlashings)
	// Report on proposer slashings.
	s.proposerSlashings(ctx, proposerSlashings)
}

func (s *Service) attesterSlashings(ctx context.Context, slashings []*spec.AttesterSlashing) {
	for _, slashing := range slashings {
		slashedIndices := intersection(slashing.Attestation1.AttestingIndices, slashing.Attestation2.AttestingIndices)
		for _, validatorIndex := range slashedIndices {
			s.log.Info().Uint64("validator_index", uint64(validatorIndex)).Msg("Validator slashed (attester)")
			if err := s.OnAttesterSlashed(ctx, validatorIndex); err != nil {
				s.log.Error().Err(err).Msg("Failed to run script")
			}
			slashingFound(ctx, validatorIndex)
		}
	}
}

func (s *Service) proposerSlashings(ctx context.Context, slashings []*spec.ProposerSlashing) {
	for _, slashing := range slashings {
		validatorIndex := slashing.SignedHeader1.Message.ProposerIndex
		s.log.Info().Uint64("validator_index", uint64(validatorIndex)).Msg("Validator slashed (proposer)")
		if err := s.OnProposerSlashed(ctx, validatorIndex); err != nil {
			s.log.Error().Err(err).Msg("Failed to run script")
		}
		slashingFound(ctx, validatorIndex)
	}
}

// intersection returns a list of items common between the two sets.
func intersection(set1 []uint64, set2 []uint64) []spec.ValidatorIndex {
	sort.Slice(set1, func(i, j int) bool { return set1[i] < set1[j] })
	sort.Slice(set2, func(i, j int) bool { return set2[i] < set2[j] })
	res := make([]spec.ValidatorIndex, 0)

	set1Pos := 0
	set2Pos := 0
	for set1Pos < len(set1) && set2Pos < len(set2) {
		switch {
		case set1[set1Pos] < set2[set2Pos]:
			set1Pos++
		case set2[set2Pos] < set1[set1Pos]:
			set2Pos++
		default:
			res = append(res, spec.ValidatorIndex(set1[set1Pos]))
			set1Pos++
			set2Pos++
		}
	}

	return res
}
