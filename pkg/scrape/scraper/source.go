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

package scraper

import (
	"context"
	"errors"

	"github.com/stretchr/testify/mock"

	"github.com/vandeefeng/zenfeed/pkg/model"
)

// --- Interface code block ---

// reader defines interface for reading from different data sources.
type reader interface {
	// Read fetches content from the data source.
	// Returns a slice of feeds and any error encountered.
	Read(ctx context.Context) ([]*model.Feed, error)
}

// --- Factory code block ---

func NewReader(config *Config, dependencies Dependencies) (reader, error) {
	if config.RSS != nil {
		return newRSSReader(config.RSS, config.Past, dependencies.KVStorage)
	}

	return nil, errors.New("no source config")
}

// --- Implementation code block ---

type mockReader struct {
	mock.Mock
}

func NewMock() *mockReader {
	return &mockReader{}
}

func (m *mockReader) Read(ctx context.Context) ([]*model.Feed, error) {
	args := m.Called(ctx)
	if feeds := args.Get(0); feeds != nil {
		return feeds.([]*model.Feed), args.Error(1)
	}

	return nil, args.Error(1)
}
