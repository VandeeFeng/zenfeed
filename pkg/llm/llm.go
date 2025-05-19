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

package llm

import (
	"context"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/mock"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/config"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/storage/kv"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
	"github.com/vandeefeng/zenfeed/pkg/util/hash"
)

// --- Interface code block ---
type LLM interface {
	component.Component
	String(ctx context.Context, messages []string) (string, error)
	EmbeddingLabels(ctx context.Context, labels model.Labels) ([][]float32, error)
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type Config struct {
	Name                  string
	Default               bool
	Provider              ProviderType
	Endpoint              string
	APIKey                string
	Model, EmbeddingModel string
	Temperature           float32
}

type ProviderType string

const (
	ProviderTypeOpenAI      ProviderType = "openai"
	ProviderTypeOpenRouter  ProviderType = "openrouter"
	ProviderTypeDeepSeek    ProviderType = "deepseek"
	ProviderTypeGemini      ProviderType = "gemini"
	ProviderTypeVolc        ProviderType = "volc" // Rename MaaS to ARK. 😄
	ProviderTypeSiliconFlow ProviderType = "siliconflow"
)

var defaultEndpoints = map[ProviderType]string{
	ProviderTypeOpenAI:      "https://api.openai.com/v1",
	ProviderTypeOpenRouter:  "https://openrouter.ai/api/v1",
	ProviderTypeDeepSeek:    "https://api.deepseek.com/v1",
	ProviderTypeGemini:      "https://generativelanguage.googleapis.com/v1beta/openai",
	ProviderTypeVolc:        "https://ark.cn-beijing.volces.com/api/v3",
	ProviderTypeSiliconFlow: "https://api.siliconflow.cn/v1",
}

func (c *Config) Validate() error { //nolint:cyclop
	if c.Name == "" {
		return errors.New("name is required")
	}

	switch c.Provider {
	case "":
		c.Provider = ProviderTypeOpenAI
	case ProviderTypeOpenAI, ProviderTypeOpenRouter, ProviderTypeDeepSeek,
		ProviderTypeGemini, ProviderTypeVolc, ProviderTypeSiliconFlow:
	default:
		return errors.Errorf("invalid provider: %s", c.Provider)
	}

	if c.Endpoint == "" {
		c.Endpoint = defaultEndpoints[c.Provider]
	}
	if c.APIKey == "" {
		return errors.New("api key is required")
	}
	if c.Model == "" && c.EmbeddingModel == "" {
		return errors.New("model or embedding model is required")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return errors.Errorf("invalid temperature: %f, should be in range [0, 2]", c.Temperature)
	}

	return nil
}

var (
	promptTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "prompt_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
	completionTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "completion_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
	totalTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "total_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
)

// --- Factory code block ---
type FactoryConfig struct {
	LLMs       []Config
	defaultLLM string
}

func (c *FactoryConfig) Validate() error {
	if len(c.LLMs) == 0 {
		return errors.New("no llm config")
	}

	for i := range c.LLMs {
		if err := (&c.LLMs[i]).Validate(); err != nil {
			return errors.Wrapf(err, "validate llm config %s", c.LLMs[i].Name)
		}
	}

	if len(c.LLMs) == 1 {
		c.LLMs[0].Default = true
		c.defaultLLM = c.LLMs[0].Name

		return nil
	}

	defaults := 0
	for _, llm := range c.LLMs {
		if llm.Default {
			c.defaultLLM = llm.Name
			defaults++
		}
	}
	if defaults > 1 {
		return errors.New("multiple llm configs are default")
	}

	return nil
}

func (c *FactoryConfig) From(app *config.App) {
	for _, llm := range app.LLMs {
		c.LLMs = append(c.LLMs, Config{
			Name:           llm.Name,
			Default:        llm.Default,
			Provider:       ProviderType(llm.Provider),
			Endpoint:       llm.Endpoint,
			APIKey:         llm.APIKey,
			Model:          llm.Model,
			EmbeddingModel: llm.EmbeddingModel,
			Temperature:    llm.Temperature,
		})
	}
}

type FactoryDependencies struct {
	KVStorage kv.Storage
}

// Factory is a factory for creating LLM instances.
// If name is empty or not found, it will return the default.
type Factory interface {
	component.Component
	config.Watcher
	Get(name string) LLM
}

func NewFactory(
	instance string,
	app *config.App,
	dependencies FactoryDependencies,
	mockOn ...component.MockOption,
) (Factory, error) {
	if len(mockOn) > 0 {
		mf := &mockFactory{}
		getCall := mf.On("Get", mock.Anything)
		getCall.Run(func(args mock.Arguments) {
			m := &mockLLM{}
			component.MockOptions(mockOn).Apply(&m.Mock)
			getCall.Return(m, nil)
		})
		mf.On("Reload", mock.Anything).Return(nil)

		return mf, nil
	}

	config := &FactoryConfig{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}
	f := &factory{
		Base: component.New(&component.BaseConfig[FactoryConfig, FactoryDependencies]{
			Name:         "LLMFactory",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		llms: make(map[string]LLM),
	}
	f.initLLMs()

	return f, nil
}

type factory struct {
	*component.Base[FactoryConfig, FactoryDependencies]

	defaultLLM LLM
	llms       map[string]LLM
	mu         sync.Mutex
}

func (f *factory) Run() error {
	for _, llm := range f.llms {
		if err := component.RunUntilReady(f.Context(), llm, 10*time.Second); err != nil {
			return errors.Wrapf(err, "run llm %s", llm.Name())
		}
	}
	f.MarkReady()
	<-f.Context().Done()

	return nil
}

func (f *factory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, llm := range f.llms {
		_ = llm.Close()
	}

	return nil
}

func (f *factory) Reload(app *config.App) error {
	newConfig := &FactoryConfig{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate config")
	}
	if reflect.DeepEqual(f.Config(), newConfig) {
		log.Debug(f.Context(), "no changes in llm config")

		return nil
	}

	// Reload the LLMs.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SetConfig(newConfig)

	// Close the old LLMs.
	for _, llm := range f.llms {
		_ = llm.Close()
	}

	// Recreate the LLMs.
	f.initLLMs()

	return nil
}

func (f *factory) Get(name string) LLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == "" {
		return f.defaultLLM
	}

	for _, llmC := range f.Config().LLMs {
		if llmC.Name != name {
			continue
		}

		if f.llms[name] == nil {
			llm := f.new(&llmC)
			f.llms[name] = llm
		}

		return f.llms[name]
	}

	return f.defaultLLM
}

func (f *factory) new(c *Config) LLM {
	switch c.Provider {
	case ProviderTypeOpenAI, ProviderTypeOpenRouter, ProviderTypeDeepSeek, ProviderTypeGemini, ProviderTypeVolc, ProviderTypeSiliconFlow: //nolint:lll
		return newCached(newOpenAI(c), f.Dependencies().KVStorage)
	default:
		return newCached(newOpenAI(c), f.Dependencies().KVStorage)
	}
}

func (f *factory) initLLMs() {
	var (
		config     = f.Config()
		llms       = make(map[string]LLM, len(config.LLMs))
		defaultLLM LLM
	)
	for _, llmC := range config.LLMs {
		llm := f.new(&llmC)
		llms[llmC.Name] = llm

		if llmC.Name == config.defaultLLM {
			defaultLLM = llm
		}
	}
	f.llms = llms
	f.defaultLLM = defaultLLM
}

type mockFactory struct {
	component.Mock
}

func (m *mockFactory) Get(name string) LLM {
	args := m.Called(name)

	return args.Get(0).(LLM)
}

func (m *mockFactory) Reload(app *config.App) error {
	args := m.Called(app)

	return args.Error(0)
}

// --- Implementation code block ---
type cached struct {
	LLM
	kvStorage kv.Storage
}

func newCached(llm LLM, kvStorage kv.Storage) LLM {
	return &cached{
		LLM:       llm,
		kvStorage: kvStorage,
	}
}

func (c *cached) String(ctx context.Context, messages []string) (string, error) {
	key := hash.Sum64s(messages)
	keyStr := strconv.FormatUint(key, 10)

	value, err := c.kvStorage.Get(ctx, keyStr)
	switch {
	case err == nil:
		return value, nil
	case errors.Is(err, kv.ErrNotFound):
		break
	default:
		return "", errors.Wrap(err, "get from kv storage")
	}

	value, err = c.LLM.String(ctx, messages)
	if err != nil {
		return "", err
	}

	if err = c.kvStorage.Set(ctx, keyStr, value, 65*time.Minute); err != nil {
		log.Error(ctx, err, "set to kv storage")
	}

	return value, nil
}

type mockLLM struct {
	component.Mock
}

func (m *mockLLM) String(ctx context.Context, messages []string) (string, error) {
	args := m.Called(ctx, messages)

	return args.Get(0).(string), args.Error(1)
}

func (m *mockLLM) EmbeddingLabels(ctx context.Context, labels model.Labels) ([][]float32, error) {
	args := m.Called(ctx, labels)

	return args.Get(0).([][]float32), args.Error(1)
}

func (m *mockLLM) Embedding(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)

	return args.Get(0).([]float32), args.Error(1)
}
