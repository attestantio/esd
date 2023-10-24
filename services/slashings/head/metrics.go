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

	"github.com/attestantio/esd/services/metrics"
	spec "github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

var metricsNamespace = "esd"

var (
	blocksProcessed prometheus.Counter
	slashings       *prometheus.CounterVec
)

func registerMetrics(ctx context.Context, monitor metrics.Service) error {
	if blocksProcessed != nil {
		// Already registered.
		return nil
	}
	if monitor == nil {
		// No monitor.
		return nil
	}
	if monitor.Presenter() == "prometheus" {
		return registerPrometheusMetrics(ctx)
	}

	return nil
}

func registerPrometheusMetrics(_ context.Context) error {
	blocksProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "blocks_processed_total",
		Help:      "Total number of blocks processed",
	})
	if err := prometheus.Register(blocksProcessed); err != nil {
		return errors.Wrap(err, "failed to register blocks_processed_total")
	}

	slashings = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "slashings_total",
		Help:      "Register of slashings found",
	}, []string{"index"})
	if err := prometheus.Register(slashings); err != nil {
		return errors.Wrap(err, "failed to register slashings")
	}

	return nil
}

func blockProcessed(_ context.Context) {
	if blocksProcessed != nil {
		blocksProcessed.Inc()
	}
}

func slashingFound(_ context.Context, index spec.ValidatorIndex) {
	if slashings != nil {
		slashings.WithLabelValues(fmt.Sprintf("%d", index)).Inc()
	}
}
