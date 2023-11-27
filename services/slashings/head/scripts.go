// Copyright © 2021, 2023 Attestant Limited.
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
	"os/exec"

	spec "github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/pkg/errors"
)

// OnProposerSlashed handles a proposer slashing event.
func (s *Service) OnProposerSlashed(_ context.Context, index spec.ValidatorIndex) error {
	if s.proposerSlashedScript == "" {
		return nil
	}

	s.log.Trace().Str("script", s.attesterSlashedScript).Msg("Calling script for slashed proposer")
	//nolint:gosec
	output, err := exec.Command(s.proposerSlashedScript, fmt.Sprintf("%d", index)).CombinedOutput()
	if err != nil {
		s.log.Warn().Str("output", string(output)).Msg("Run information")
		return errors.Wrap(err, "failed to run proposer slashing script")
	}

	return nil
}

// OnAttesterSlashed handles an attester slashing event.
func (s *Service) OnAttesterSlashed(_ context.Context, index spec.ValidatorIndex) error {
	if s.attesterSlashedScript == "" {
		return nil
	}

	s.log.Info().Str("script", s.attesterSlashedScript).Msg("Calling script for slashed attester")
	//nolint:gosec
	output, err := exec.Command(s.attesterSlashedScript, fmt.Sprintf("%d", index)).CombinedOutput()
	if err != nil {
		s.log.Warn().Str("output", string(output)).Msg("Run information")
		return errors.Wrap(err, "failed to run attester slashing script")
	}

	return nil
}
