// Copyright © 2021 - 2024 Attestant Limited.
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

	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/attestantio/go-eth2-client/api"
	apiv1 "github.com/attestantio/go-eth2-client/api/v1"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	zerologger "github.com/rs/zerolog/log"
)

// Service is slashings services that watches blocks for slashings.
type Service struct {
	log                   zerolog.Logger
	eth2Client            eth2client.Service
	attesterSlashedScript string
	proposerSlashedScript string
}

// New creates a new service.
func New(ctx context.Context, params ...Parameter) (*Service, error) {
	parameters, err := parseAndCheckParameters(params...)
	if err != nil {
		return nil, errors.Wrap(err, "problem with parameters")
	}

	// Set logging.
	log := zerologger.With().Str("service", "slashings").Str("impl", "head").Logger()
	if parameters.logLevel != log.GetLevel() {
		log = log.Level(parameters.logLevel)
	}

	svc := &Service{
		log:                   log,
		eth2Client:            parameters.eth2Client,
		attesterSlashedScript: parameters.attesterSlashedScript,
		proposerSlashedScript: parameters.proposerSlashedScript,
	}

	if parameters.block != "" {
		// Require running for a specific slot (for test purposes).
		signedBeaconBlockProvider, isProvider := parameters.eth2Client.(eth2client.SignedBeaconBlockProvider)
		if !isProvider {
			return nil, errors.New("client does not provide signed beacon blocks")
		}

		blockResponse, err := signedBeaconBlockProvider.SignedBeaconBlock(ctx, &api.SignedBeaconBlockOpts{
			Block: parameters.block,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain block")
		}
		block := blockResponse.Data

		slot, err := block.Slot()
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain slot")
		}
		root, err := block.Root()
		if err != nil {
			return nil, errors.Wrap(err, "failed to obtain root")
		}
		svc.OnHeadUpdated(ctx, slot, root)
		// Service is not initialised, so do not return it.
		//nolint:nilnil
		return nil, nil
	}

	if parameters.monitor != nil {
		if err := registerMetrics(ctx, parameters.monitor); err != nil {
			return nil, errors.Wrap(err, "failed to register metrics")
		}
	}

	eventsProvider, isEventsProvider := svc.eth2Client.(eth2client.EventsProvider)
	if !isEventsProvider {
		return nil, errors.New("eth2 client is not an events provider")
	}
	if err := eventsProvider.Events(ctx, []string{"head"}, func(event *apiv1.Event) {
		if event.Data == nil {
			return
		}

		eventData, isEventData := event.Data.(*apiv1.HeadEvent)
		if !isEventData {
			svc.log.Error().Msg("event data is not from a head event; cannot process")
			return
		}

		svc.OnHeadUpdated(ctx, eventData.Slot, eventData.Block)
		blockProcessed(ctx)
	}); err != nil {
		return nil, errors.Wrap(err, "failed to configure head event feed")
	}

	return svc, nil
}
