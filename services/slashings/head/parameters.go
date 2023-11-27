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
	"errors"

	"github.com/attestantio/esd/services/metrics"
	eth2client "github.com/attestantio/go-eth2-client"
	"github.com/rs/zerolog"
)

type parameters struct {
	logLevel              zerolog.Level
	eth2Client            eth2client.Service
	monitor               metrics.Service
	attesterSlashedScript string
	proposerSlashedScript string
	block                 string
}

// Parameter is the interface for service parameters.
type Parameter interface {
	apply(p *parameters)
}

type parameterFunc func(*parameters)

func (f parameterFunc) apply(p *parameters) {
	f(p)
}

// WithLogLevel sets the log level for the module.
func WithLogLevel(logLevel zerolog.Level) Parameter {
	return parameterFunc(func(p *parameters) {
		p.logLevel = logLevel
	})
}

// WithMonitor sets the monitor for this module.
func WithMonitor(monitor metrics.Service) Parameter {
	return parameterFunc(func(p *parameters) {
		p.monitor = monitor
	})
}

// WithETH2Client sets the Ethereum 2 client for this module.
func WithETH2Client(eth2Client eth2client.Service) Parameter {
	return parameterFunc(func(p *parameters) {
		p.eth2Client = eth2Client
	})
}

// WithAttesterSlashedScript sets the script when an attester is slashed.
func WithAttesterSlashedScript(script string) Parameter {
	return parameterFunc(func(p *parameters) {
		p.attesterSlashedScript = script
	})
}

// WithProposerSlashedScript sets the script when an proposer is slashed.
func WithProposerSlashedScript(script string) Parameter {
	return parameterFunc(func(p *parameters) {
		p.proposerSlashedScript = script
	})
}

// WithBlock sets the block to run against.
func WithBlock(block string) Parameter {
	return parameterFunc(func(p *parameters) {
		p.block = block
	})
}

// parseAndCheckParameters parses and checks parameters to ensure that mandatory parameters are present and correct.
func parseAndCheckParameters(params ...Parameter) (*parameters, error) {
	parameters := parameters{
		logLevel: zerolog.GlobalLevel(),
	}
	for _, p := range params {
		if params != nil {
			p.apply(&parameters)
		}
	}

	if parameters.eth2Client == nil {
		return nil, errors.New("no Ethereum 2 client specified")
	}

	return &parameters, nil
}
