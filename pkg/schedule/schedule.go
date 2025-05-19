// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package schedule

import (
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/config"
	"github.com/vandeefeng/zenfeed/pkg/schedule/rule"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Scheduler interface {
	component.Component
	config.Watcher
}

type Config struct {
	Rules []rule.Config
}

func (c *Config) Validate() error {
	for _, rule := range c.Rules {
		if err := (&rule).Validate(); err != nil {
			return errors.Wrap(err, "validate rule")
		}
	}

	return nil
}

func (c *Config) From(app *config.App) *Config {
	c.Rules = make([]rule.Config, len(app.Scheduls.Rules))
	for i, r := range app.Scheduls.Rules {
		c.Rules[i] = rule.Config{
			Name:          r.Name,
			Query:         r.Query,
			Threshold:     r.Threshold,
			LabelFilters:  r.LabelFilters,
			EveryDay:      r.EveryDay,
			WatchInterval: time.Duration(r.WatchInterval),
		}
	}

	return c
}

type Dependencies struct {
	RuleFactory rule.Factory
	FeedStorage feed.Storage
	Out         chan<- *rule.Result
}

// --- Factory code block ---
type Factory component.Factory[Scheduler, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Scheduler, config.App, Dependencies](
			func(instance string, app *config.App, dependencies Dependencies) (Scheduler, error) {
				m := &mockScheduler{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Scheduler, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Scheduler, error) {
	config := &Config{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	s := &scheduler{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         instance,
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		rules: make(map[string]rule.Rule, len(config.Rules)),
	}

	for i := range config.Rules {
		r := &config.Rules[i]
		rule, err := s.newRule(r)
		if err != nil {
			return nil, errors.Wrapf(err, "create rule %s", r.Name)
		}
		s.rules[r.Name] = rule
	}

	return s, nil
}

// --- Implementation code block ---
type scheduler struct {
	*component.Base[Config, Dependencies]

	rules map[string]rule.Rule
}

func (s *scheduler) Run() (err error) {
	ctx := telemetry.StartWith(s.Context(), append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "Run")...)
	defer func() { telemetry.End(ctx, err) }()

	for _, r := range s.rules {
		if err := component.RunUntilReady(ctx, r, 10*time.Second); err != nil {
			return errors.Wrapf(err, "running rule %s", r.Config().Name)
		}
	}

	s.MarkReady()
	<-ctx.Done()

	return nil
}

func (s *scheduler) Reload(app *config.App) error {
	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate config")
	}
	if reflect.DeepEqual(s.Config(), newConfig) {
		log.Debug(s.Context(), "no changes in schedule config")

		return nil
	}

	newRules := make(map[string]rule.Rule, len(newConfig.Rules))

	if err := s.runOrRestartRules(newConfig, newRules); err != nil {
		return errors.Wrap(err, "run or restart rules")
	}
	if err := s.stopObsoleteRules(newRules); err != nil {
		return errors.Wrap(err, "stop obsolete rules")
	}

	s.rules = newRules
	s.SetConfig(newConfig)

	return nil
}

func (s *scheduler) Close() error {
	if err := s.Base.Close(); err != nil {
		return errors.Wrap(err, "close base")
	}

	// Stop all rules.
	for _, r := range s.rules {
		_ = r.Close()
	}

	return nil
}

func (s *scheduler) newRule(config *rule.Config) (rule.Rule, error) {
	return s.Dependencies().RuleFactory.New(config.Name, config, rule.Dependencies{
		FeedStorage: s.Dependencies().FeedStorage,
		Out:         s.Dependencies().Out,
	})
}

func (s *scheduler) runOrRestartRules(config *Config, newRules map[string]rule.Rule) error {
	for _, r := range config.Rules {
		// Close or reuse existing rule.
		if existing, exists := s.rules[r.Name]; exists {
			if reflect.DeepEqual(existing.Config(), r) {
				newRules[r.Name] = existing

				continue
			}

			if err := existing.Close(); err != nil {
				return errors.Wrap(err, "close existing rule")
			}
		}

		// Create & Run new/updated rule.
		newRule, err := s.newRule(&r)
		if err != nil {
			return errors.Wrap(err, "create rule")
		}
		newRules[r.Name] = newRule
		if err := component.RunUntilReady(s.Context(), newRule, 10*time.Second); err != nil {
			return errors.Wrapf(err, "running rule %s", r.Name)
		}
	}

	return nil
}

func (s *scheduler) stopObsoleteRules(newRules map[string]rule.Rule) error {
	var lastErr error
	for name, r := range s.rules {
		if _, exists := newRules[name]; !exists {
			if err := r.Close(); err != nil {
				lastErr = errors.Wrap(err, "close obsolete rule")
			}
		}
	}

	return lastErr
}

type mockScheduler struct {
	component.Mock
}

func (m *mockScheduler) Reload(app *config.App) error {
	args := m.Called(app)

	return args.Error(0)
}
