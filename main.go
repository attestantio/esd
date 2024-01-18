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

package main

import (
	"context"
	"fmt"
	"net/http"

	// #nosec G108
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/attestantio/esd/services/metrics"
	nullmetrics "github.com/attestantio/esd/services/metrics/null"
	prometheusmetrics "github.com/attestantio/esd/services/metrics/prometheus"
	headslashings "github.com/attestantio/esd/services/slashings/head"
	"github.com/attestantio/esd/util"
	"github.com/aws/aws-sdk-go/aws/credentials"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	zerologger "github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	e2types "github.com/wealdtech/go-eth2-types/v2"
	majordomo "github.com/wealdtech/go-majordomo"
	asmconfidant "github.com/wealdtech/go-majordomo/confidants/asm"
	directconfidant "github.com/wealdtech/go-majordomo/confidants/direct"
	fileconfidant "github.com/wealdtech/go-majordomo/confidants/file"
	gsmconfidant "github.com/wealdtech/go-majordomo/confidants/gsm"
	standardmajordomo "github.com/wealdtech/go-majordomo/standard"
)

// ReleaseVersion is the release version for the code.
var ReleaseVersion = "1.2.4"

func main() {
	os.Exit(main2())
}

func main2() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := fetchConfig(); err != nil {
		zerologger.Error().Err(err).Msg("Failed to fetch configuration")
		return 1
	}

	if err := initLogging(); err != nil {
		log.Error().Err(err).Msg("Failed to initialise logging")
		return 1
	}

	exit, err := runCommands(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to run command")
		return 1
	}
	if exit {
		return 0
	}

	logModules()
	log.Info().Str("version", ReleaseVersion).Str("commit_hash", util.CommitHash()).Msg("Starting ESD")

	initProfiling()

	runtime.GOMAXPROCS(runtime.NumCPU() * 8)

	if err := e2types.InitBLS(); err != nil {
		log.Error().Err(err).Msg("Failed to initialise BLS library")
		return 1
	}

	log.Trace().Msg("Starting metrics service")
	monitor, err := startMonitor(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to start metrics service")
		return 1
	}
	if err := registerMetrics(ctx, monitor); err != nil {
		log.Error().Err(err).Msg("Failed to register metrics")
	}
	setRelease(ctx, ReleaseVersion)
	setReady(ctx, false)

	majordomo, err := initMajordomo(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialise majordomo")
		return 1
	}

	if err := startServices(ctx, monitor, majordomo); err != nil {
		log.Error().Err(err).Msg("Failed to initialise services")
		return 1
	}
	setReady(ctx, true)

	log.Info().Msg("All services operational")

	// Wait for signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	for {
		sig := <-sigCh
		if sig == syscall.SIGINT || sig == syscall.SIGTERM || sig == os.Interrupt || sig == os.Kill {
			break
		}
	}

	log.Info().Msg("Stopping ESD")

	return 0
}

// fetchConfig fetches configuration from various sources.
func fetchConfig() error {
	pflag.String("base-dir", "", "base directory for configuration files")
	pflag.Bool("version", false, "show version and exit")
	pflag.String("log-level", "info", "minimum level of messsages to log")
	pflag.String("log-file", "", "redirect log output to a file")
	pflag.String("profile-address", "", "Address on which to run Go profile server")
	pflag.String("tracing-address", "", "Address to which to send tracing data")
	pflag.String("eth2client.address", "", "Address for beacon node")
	pflag.Duration("eth2client.timeout", 2*time.Minute, "Timeout for beacon node requests")
	pflag.String("slashings.attester-slashed-script", "", "Script to run when attester is slashed")
	pflag.String("slashings.proposer-slashed-script", "", "Script to run when proposer is slashed")
	pflag.Bool("test-scripts", false, "Test scripts using validator index 12345678 and exit")
	pflag.String("test-block", "", "Test scripts using supplied block and exit")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return errors.Wrap(err, "failed to bind pflags to viper")
	}

	if viper.GetString("base-dir") != "" {
		// User-defined base directory.
		viper.AddConfigPath(resolvePath(""))
		viper.SetConfigName("esd")
	} else {
		// Home directory.
		home, err := homedir.Dir()
		if err != nil {
			return errors.Wrap(err, "failed to obtain home directory")
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".esd")
	}

	// Environment settings.
	viper.SetEnvPrefix("ESD")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if !errors.Is(err, &viper.ConfigFileNotFoundError{}) {
			return errors.Wrap(err, "failed to read configuration file")
		}
	}

	return nil
}

// initProfiling initialises the profiling server.
func initProfiling() {
	profileAddress := viper.GetString("profile-address")
	if profileAddress != "" {
		go func() {
			log.Info().Str("profile_address", profileAddress).Msg("Starting profile server")
			runtime.SetMutexProfileFraction(1)
			server := &http.Server{
				Addr:              profileAddress,
				ReadHeaderTimeout: 5 * time.Second,
			}
			if err := server.ListenAndServe(); err != nil {
				log.Warn().Str("profile_address", profileAddress).Err(err).Msg("Failed to run profile server")
			}
		}()
	}
}

func startServices(ctx context.Context, monitor metrics.Service, _ majordomo.Service) error {
	log.Trace().Msg("Starting Ethereum 2 client service")
	eth2Client, err := fetchClient(ctx, viper.GetString("eth2client.address"))
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to fetch client %q", viper.GetString("eth2client.address")))
	}

	_, err = headslashings.New(ctx,
		headslashings.WithLogLevel(util.LogLevel("slashings")),
		headslashings.WithMonitor(monitor),
		headslashings.WithETH2Client(eth2Client),
		headslashings.WithAttesterSlashedScript(viper.GetString("slashings.attester-slashed-script")),
		headslashings.WithProposerSlashedScript(viper.GetString("slashings.proposer-slashed-script")),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create slashings service")
	}

	return nil
}

// runCommands returns true if it ran a command and requests exit.
func runCommands(ctx context.Context) (bool, error) {
	if viper.GetBool("version") {
		fmt.Printf("%s\n", ReleaseVersion)
		return true, nil
	}

	if viper.GetBool("test-scripts") {
		return runTestScripts(ctx)
	}

	if viper.GetString("test-block") != "" {
		return runTestBlock(ctx)
	}

	return false, nil
}

func runTestScripts(ctx context.Context) (bool, error) {
	eth2Client, err := fetchClient(ctx, viper.GetString("eth2client.address"))
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to fetch client %q", viper.GetString("eth2client.address")))
	}

	slashings, err := headslashings.New(ctx,
		headslashings.WithLogLevel(util.LogLevel("slashings")),
		headslashings.WithETH2Client(eth2Client),
		headslashings.WithAttesterSlashedScript(viper.GetString("slashings.attester-slashed-script")),
		headslashings.WithProposerSlashedScript(viper.GetString("slashings.proposer-slashed-script")),
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to create slashings service")
	}

	if viper.GetString("slashings.attester-slashed-script") != "" {
		fmt.Fprintf(os.Stdout, "Testing attester slashing script with validator index 12345678\n")
		if err := slashings.OnAttesterSlashed(ctx, 12345678); err != nil {
			fmt.Fprintf(os.Stdout, "Attester slashing script failed: %v\n", err)
			return true, nil
		}
	} else {
		fmt.Println("No attester slashing script")
	}

	if viper.GetString("slashings.proposer-slashed-script") != "" {
		fmt.Fprintf(os.Stdout, "Testing proposer slashing script with validator index 12345678\n")
		if err := slashings.OnProposerSlashed(ctx, 12345678); err != nil {
			fmt.Fprintf(os.Stdout, "Proposer slashing script failed: %v\n", err)
			return true, nil
		}
	} else {
		fmt.Fprintf(os.Stdout, "No proposer slashing script\n")
	}

	return true, nil
}

func runTestBlock(ctx context.Context) (bool, error) {
	eth2Client, err := fetchClient(ctx, viper.GetString("eth2client.address"))
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to fetch client %q", viper.GetString("eth2client.address")))
	}

	_, err = headslashings.New(ctx,
		headslashings.WithLogLevel(util.LogLevel("slashings")),
		headslashings.WithETH2Client(eth2Client),
		headslashings.WithAttesterSlashedScript(viper.GetString("slashings.attester-slashed-script")),
		headslashings.WithProposerSlashedScript(viper.GetString("slashings.proposer-slashed-script")),
		headslashings.WithBlock(viper.GetString("test-block")),
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to create slashings service")
	}

	return true, nil
}

func logModules() {
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		log.Trace().Str("path", buildInfo.Path).Msg("Main package")
		for _, dep := range buildInfo.Deps {
			path := dep.Path
			if dep.Replace != nil {
				path = dep.Replace.Path
			}
			log.Trace().Str("path", path).Str("version", dep.Version).Msg("Dependency")
		}
	}
}

// resolvePath resolves a potentially relative path to an absolute path.
func resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	baseDir := viper.GetString("base-dir")
	if baseDir == "" {
		homeDir, err := homedir.Dir()
		if err != nil {
			log.Fatal().Err(err).Msg("Could not determine a home directory")
		}
		baseDir = homeDir
	}

	return filepath.Join(baseDir, path)
}

func startMonitor(ctx context.Context) (metrics.Service, error) {
	log.Trace().Msg("Starting metrics service")
	var monitor metrics.Service
	if viper.Get("metrics.prometheus") != nil {
		var err error
		monitor, err = prometheusmetrics.New(ctx,
			prometheusmetrics.WithLogLevel(util.LogLevel("metrics.prometheus")),
			prometheusmetrics.WithAddress(viper.GetString("metrics.prometheus.listen-address")),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start prometheus metrics service")
		}
		log.Info().Str("listen_address", viper.GetString("metrics.prometheus.listen-address")).Msg("Started prometheus metrics service")
	} else {
		log.Debug().Msg("No metrics service supplied; monitor not starting")
		monitor = &nullmetrics.Service{}
	}

	return monitor, nil
}

func initMajordomo(ctx context.Context) (majordomo.Service, error) {
	majordomo, err := standardmajordomo.New(ctx,
		standardmajordomo.WithLogLevel(util.LogLevel("majordomo")),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create majordomo service")
	}

	directConfidant, err := directconfidant.New(ctx,
		directconfidant.WithLogLevel(util.LogLevel("majordomo.confidants.direct")),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create direct confidant")
	}
	if err := majordomo.RegisterConfidant(ctx, directConfidant); err != nil {
		return nil, errors.Wrap(err, "failed to register direct confidant")
	}

	fileConfidant, err := fileconfidant.New(ctx,
		fileconfidant.WithLogLevel(util.LogLevel("majordomo.confidants.file")),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create file confidant")
	}
	if err := majordomo.RegisterConfidant(ctx, fileConfidant); err != nil {
		return nil, errors.Wrap(err, "failed to register file confidant")
	}

	if viper.GetString("majordomo.asm.region") != "" {
		var asmCredentials *credentials.Credentials
		if viper.GetString("majordomo.asm.id") != "" {
			asmCredentials = credentials.NewStaticCredentials(
				viper.GetString("majordomo.asm.id"),
				viper.GetString("majordomo.asm.secret"),
				"",
			)
		}
		asmConfidant, err := asmconfidant.New(ctx,
			asmconfidant.WithLogLevel(util.LogLevel("majordomo.confidants.asm")),
			asmconfidant.WithCredentials(asmCredentials),
			asmconfidant.WithRegion(viper.GetString("majordomo.asm.region")),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create AWS secrets manager confidant")
		}
		if err := majordomo.RegisterConfidant(ctx, asmConfidant); err != nil {
			return nil, errors.Wrap(err, "failed to register AWS secrets manager confidant")
		}
	}

	if viper.GetString("majordomo.gsm.credentials") != "" {
		gsmConfidant, err := gsmconfidant.New(ctx,
			gsmconfidant.WithLogLevel(util.LogLevel("majordomo.confidants.gsm")),
			gsmconfidant.WithCredentialsPath(resolvePath(viper.GetString("majordomo.gsm.credentials"))),
			gsmconfidant.WithProject(viper.GetString("majordomo.gsm.project")),
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create Google secret manager confidant")
		}
		if err := majordomo.RegisterConfidant(ctx, gsmConfidant); err != nil {
			return nil, errors.Wrap(err, "failed to register Google secret manager confidant")
		}
	}

	return majordomo, nil
}
