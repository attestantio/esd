// Copyright © 2021 Attestant Limited.
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
)

// OnProposerSlashed handles a proposer slashing event.
func (s *Service) OnProposerSlashed(_ context.Context, index spec.ValidatorIndex) error {
	if s.proposerSlashedScript == "" {
		return nil
	}

	log.Trace().Str("script", s.attesterSlashedScript).Msg("Calling script")
	// #nosec G204
	output, err := exec.Command(s.proposerSlashedScript, fmt.Sprintf("%d", index)).CombinedOutput()
	if err != nil {
		log.Warn().Str("output", string(output)).Msg("Run information")
	}

	return err
}

// OnAttesterSlashed handles an attester slashing event.
func (s *Service) OnAttesterSlashed(_ context.Context, index spec.ValidatorIndex) error {
	if s.attesterSlashedScript == "" {
		return nil
	}

	log.Info().Str("script", s.attesterSlashedScript).Msg("Calling script")
	// #nosec G204
	output, err := exec.Command(s.attesterSlashedScript, fmt.Sprintf("%d", index)).CombinedOutput()
	if err != nil {
		log.Warn().Str("output", string(output)).Msg("Run information")
	}

	return err
}
