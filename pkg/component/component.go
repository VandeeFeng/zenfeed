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

package component

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/mock"

	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

// Global is the instance name for the global component.
const Global = "Global"

// Component is the interface for a component.
// It is used to start, stop and monitor a component.
// A component means it is runnable, has some async work to do.
// ALL exported biz structs MUST implement this interface.
type Component interface {
	// Name returns the name of the component. e.g. "KVStorage".
	// It SHOULD be unique between different components.
	// It will used as the telemetry info. e.g. log, metrics, etc.
	Name() string
	// Instance returns the instance name of the component. e.g. "kvstorage-1".
	// It SHOULD be unique between different instances of the same component.
	// It will used as the telemetry info. e.g. log, metrics, etc.
	Instance() string
	// Run starts the component.
	// It blocks until the component is closed.
	// It MUST be called only once.
	Run() (err error)
	// Ready returns a channel that is closed when the component is ready.
	// Returns a chan to notify the component is ready when Run is called.
	Ready() (notify <-chan struct{})
	// Close closes the component.
	Close() (err error)
}

// Base is the base implementation of a component.
// It provides partial, default implementations of the Component interface.
// It SHOULD BE used as an embedded field in the actual component implementation.
type Base[Config any, Dependencies any] struct {
	baseConfig      *BaseConfig[Config, Dependencies]
	telemetryLabels telemetry.Labels
	mu              sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	ch     chan struct{}
}

type BaseConfig[Config any, Dependencies any] struct {
	Name                      string
	Instance                  string
	AdditionalTelemetryLabels telemetry.Labels
	Config                    *Config
	Dependencies              Dependencies
}

func New[Config any, Dependencies any](config *BaseConfig[Config, Dependencies]) *Base[Config, Dependencies] {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan struct{})
	telemetryLabels := telemetry.Labels{
		telemetrymodel.KeyComponent, config.Name,
		telemetrymodel.KeyComponentInstance, config.Instance,
	}
	telemetryLabels = append(telemetryLabels, config.AdditionalTelemetryLabels...)

	return &Base[Config, Dependencies]{
		telemetryLabels: telemetryLabels,
		baseConfig:      config,
		ctx:             ctx,
		cancel:          cancel,
		ch:              ch,
	}
}

func (c *Base[Config, Dependencies]) Name() string {
	return c.baseConfig.Name
}

func (c *Base[Config, Dependencies]) Instance() string {
	return c.baseConfig.Instance
}

func (c *Base[Config, Dependencies]) TelemetryLabels() telemetry.Labels {
	return c.telemetryLabels
}

func (c *Base[Config, Dependencies]) TelemetryLabelsID() prometheus.Labels {
	return prometheus.Labels{
		telemetrymodel.KeyComponent:         c.telemetryLabels.Get(telemetrymodel.KeyComponent).(string),
		telemetrymodel.KeyComponentInstance: c.telemetryLabels.Get(telemetrymodel.KeyComponentInstance).(string),
	}
}

func (c *Base[Config, Dependencies]) TelemetryLabelsIDFields() []string {
	return []string{
		c.telemetryLabels.Get(telemetrymodel.KeyComponent).(string),
		c.telemetryLabels.Get(telemetrymodel.KeyComponentInstance).(string),
	}
}

func (c *Base[Config, Dependencies]) Config() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.baseConfig.Config
}

func (c *Base[Config, Dependencies]) SetConfig(config *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseConfig.Config = config
}

func (c *Base[Config, Dependencies]) Dependencies() Dependencies {
	return c.baseConfig.Dependencies
}

func (c *Base[Config, Dependencies]) Context() context.Context {
	return c.ctx
}

func (c *Base[Config, Dependencies]) Run() error {
	c.MarkReady()
	<-c.ctx.Done()

	return nil
}

func (c *Base[Config, Dependencies]) MarkReady() {
	close(c.ch)
}

func (c *Base[Config, Dependencies]) Ready() <-chan struct{} {
	return c.ch
}

func (c *Base[Config, Dependencies]) Close() error {
	c.cancel()
	telemetry.CloseMetrics(c.TelemetryLabelsID())
	log.Info(c.Context(), "component closed", c.TelemetryLabels()...)

	return nil
}

type Factory[ComponentImpl Component, Config any, Dependencies any] interface {
	New(instance string, config *Config, dependencies Dependencies) (ComponentImpl, error)
}

type FactoryFunc[ComponentImpl Component, Config any, Dependencies any] func(
	instance string,
	config *Config,
	dependencies Dependencies,
) (ComponentImpl, error)

func (f FactoryFunc[ComponentImpl, Config, Dependencies]) New(
	instance string,
	config *Config,
	dependencies Dependencies,
) (ComponentImpl, error) {
	return f(instance, config, dependencies)
}

type Mock struct {
	mock.Mock
}

func (m *Mock) Name() string {
	return m.Called().String(0)
}
func (m *Mock) Instance() string {
	return m.Called().String(0)
}
func (m *Mock) Run() error {
	return m.Called().Error(0)
}
func (m *Mock) Ready() <-chan struct{} {
	return m.Called().Get(0).(<-chan struct{})
}
func (m *Mock) Close() error {
	return m.Called().Error(0)
}

type MockOption func(m *mock.Mock)

type MockOptions []MockOption

func (m MockOptions) Apply(mock *Mock) {
	for _, opt := range m {
		opt(&mock.Mock)
	}
}

func RunUntilReady(waitCtx context.Context, component Component, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- component.Run()
	}()

	select {
	case <-component.Ready():
		log.Info(waitCtx, "component run and ready",
			telemetrymodel.KeyComponent, component.Name(),
			telemetrymodel.KeyComponentInstance, component.Instance(),
		)

		return nil
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		return errors.New("component not ready after timeout")
	case <-waitCtx.Done():
		return waitCtx.Err()
	}
}

type Group []Component

func Run(ctx context.Context, groups ...Group) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start groups in order.
	runningErrCh := make(chan error, 1)
	for i, group := range groups {
		if err := startGroup(ctx, group, runningErrCh); err != nil {
			stopGroups(groups, i)

			return err
		}
	}

	// All groups started successfully, wait for any component to fail or context to be canceled.
	select {
	case err := <-runningErrCh:
		stopGroups(groups, len(groups)-1)

		return err

	case <-ctx.Done():
		stopGroups(groups, len(groups)-1)

		return nil
	}
}

func startGroup(ctx context.Context, group Group, runningErrCh chan error) error {
	gCtx := log.With(ctx, telemetrymodel.KeyComponent, "group")
	log.Info(gCtx, "starting group", "components", len(group))

	// Start all components in current group concurrently.
	startComponents(gCtx, group, runningErrCh)

	// Wait for all components to be ready or error.
	return waitForGroupReady(ctx, group, runningErrCh)
}

func startComponents(ctx context.Context, group Group, runningErrCh chan error) {
	for _, comp := range group {
		go func(c Component) {
			log.Info(ctx, "starting component",
				telemetrymodel.KeyComponent, c.Name(),
				telemetrymodel.KeyComponentInstance, c.Instance(),
			)
			if err := c.Run(); err != nil {
				select {
				case runningErrCh <- err:
				default:
				}
			}
			log.Info(ctx, "component exited",
				telemetrymodel.KeyComponent, c.Name(),
				telemetrymodel.KeyComponentInstance, c.Instance(),
			)
		}(comp)
	}
}

func waitForGroupReady(ctx context.Context, group Group, runningErrCh chan error) error {
	for _, comp := range group {
		select {
		case <-comp.Ready():
			log.Info(ctx, "component run and ready",
				telemetrymodel.KeyComponent, comp.Name(),
				telemetrymodel.KeyComponentInstance, comp.Instance(),
			)
		case err := <-runningErrCh:
			return err
		case <-time.After(30 * time.Second):
			return errors.New("not ready after 30 seconds")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func stopGroups(groups []Group, runAt int) {
	for i := runAt; i >= 0; i-- {
		stopGroup(groups[i])
	}
}

func stopGroup(group Group) {
	var wg sync.WaitGroup
	for _, comp := range group {
		wg.Add(1)
		go func(c Component) {
			defer wg.Done()
			_ = c.Close() // Ignore close error.
		}(comp)
	}
	wg.Wait()
}
