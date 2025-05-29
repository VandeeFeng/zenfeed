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

package kv

import (
	"context"
	"strings"
	"time"

	"github.com/nutsdb/nutsdb"
	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/config"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Storage interface {
	component.Component
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

var ErrNotFound = errors.New("not found")

type Config struct {
	Dir string
}

const subDir = "kv"

func (c *Config) Validate() error {
	if c.Dir == "" {
		c.Dir = "./data/" + subDir
	}

	return nil
}

func (c *Config) From(app *config.App) *Config {
	c.Dir = app.Storage.Dir

	return c
}

type Dependencies struct{}

// --- Factory code block ---
type Factory component.Factory[Storage, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Storage, config.App, Dependencies](
			func(instance string, config *config.App, dependencies Dependencies) (Storage, error) {
				m := &mockKV{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Storage, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Storage, error) {
	config := &Config{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	return &kv{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "KVStorage",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
	}, nil
}

// --- Implementation code block ---
type kv struct {
	*component.Base[Config, Dependencies]
	db *nutsdb.DB
}

func (k *kv) Run() error {
	db, err := nutsdb.Open(
		nutsdb.DefaultOptions,
		nutsdb.WithDir(k.Config().Dir),
		nutsdb.WithSyncEnable(false),
	)
	if err != nil {
		return errors.Wrap(err, "open db")
	}
	if err := db.Update(func(tx *nutsdb.Tx) error {
		if !tx.ExistBucket(nutsdb.DataStructureBTree, bucket) {
			return tx.NewBucket(nutsdb.DataStructureBTree, bucket)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "create bucket")
	}
	k.db = db

	k.MarkReady()
	<-k.Context().Done()

	return nil
}

func (k *kv) Close() error {
	if err := k.Base.Close(); err != nil {
		return errors.Wrap(err, "close base")
	}

	return k.db.Close()
}

const bucket = "0"

func (k *kv) Get(ctx context.Context, key string) (value string, err error) {
	ctx = telemetry.StartWith(ctx, append(k.TelemetryLabels(), telemetrymodel.KeyOperation, "Get")...)
	defer func() {
		telemetry.End(ctx, func() error {
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}

			return nil
		}())
	}()

	var b []byte
	err = k.db.View(func(tx *nutsdb.Tx) error {
		b, err = tx.Get(bucket, []byte(key))

		return err
	})
	switch {
	case err == nil:
		return string(b), nil
	case errors.Is(err, nutsdb.ErrNotFoundKey):
		return "", ErrNotFound
	case strings.Contains(err.Error(), "key not found"):
		return "", ErrNotFound
	default:
		return "", err
	}
}

func (k *kv) Set(ctx context.Context, key string, value string, ttl time.Duration) (err error) {
	ctx = telemetry.StartWith(ctx, append(k.TelemetryLabels(), telemetrymodel.KeyOperation, "Set")...)
	defer func() { telemetry.End(ctx, err) }()

	return k.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Put(bucket, []byte(key), []byte(value), uint32(ttl.Seconds()))
	})
}

func (k *kv) Delete(ctx context.Context, key string) (err error) {
	ctx = telemetry.StartWith(ctx, append(k.TelemetryLabels(), telemetrymodel.KeyOperation, "Delete")...)
	defer func() { telemetry.End(ctx, err) }()

	err = k.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Delete(bucket, []byte(key))
	})
	if errors.Is(err, nutsdb.ErrNotFoundKey) || strings.Contains(err.Error(), "key not found") {
		return ErrNotFound
	}
	return err
}

type mockKV struct {
	component.Mock
}

func (m *mockKV) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)

	return args.String(0), args.Error(1)
}

func (m *mockKV) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)

	return args.Error(0)
}

func (m *mockKV) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}
