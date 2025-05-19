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

package rule

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
)

// --- Interface code block ---
type Rule interface {
	component.Component
	Config() *Config
}

type Config struct {
	Name         string
	Query        string
	Threshold    float32
	LabelFilters []string

	// Periodic type.
	EveryDay   string // e.g. "00:00~23:59", or "-22:00~7:00" (yesterday 22:00 to today 07:00)
	start, end time.Time
	crossDay   bool

	// Watch type.
	WatchInterval time.Duration
}

var (
	timeSep             = "~"
	timeYesterdayPrefix = "-"
	timeFmt             = "15:04"
)

func (c *Config) Validate() error { //nolint:cyclop,gocognit
	if c.Name == "" {
		return errors.New("name is required")
	}
	if c.Query != "" && utf8.RuneCountInString(c.Query) < 5 {
		return errors.New("query must be at least 5 characters")
	}
	if c.Threshold == 0 {
		c.Threshold = 0.6
	}
	if c.Threshold < 0 || c.Threshold > 1 {
		return errors.New("threshold must be between 0 and 1")
	}
	if c.EveryDay != "" && c.WatchInterval != 0 {
		return errors.New("every_day and watch_interval cannot both be set")
	}
	switch c.EveryDay {
	case "":
		if c.WatchInterval < 10*time.Minute {
			c.WatchInterval = 10 * time.Minute
		}
	default:
		times := strings.Split(c.EveryDay, timeSep)
		if len(times) != 2 {
			return errors.New("every_day must be in format 'start~end'")
		}

		start, end := strings.TrimSpace(times[0]), strings.TrimSpace(times[1])
		isYesterday := strings.HasPrefix(start, timeYesterdayPrefix)
		if isYesterday {
			start = start[1:] // Remove the "-" prefix
			c.crossDay = true
		}

		// Parse start time.
		startTime, err := time.ParseInLocation(timeFmt, start, time.Local)
		if err != nil {
			return errors.Wrap(err, "parse start time")
		}

		// Parse end time.
		endTime, err := time.ParseInLocation(timeFmt, end, time.Local)
		if err != nil {
			return errors.Wrap(err, "parse end time")
		}

		// For non-yesterday time range, end time must be after start time.
		if !isYesterday && endTime.Before(startTime) {
			return errors.New("end time must be after start time")
		}

		c.start, c.end = startTime, endTime
	}

	return nil
}

type Dependencies struct {
	FeedStorage feed.Storage
	Out         chan<- *Result
}

type Result struct {
	Rule  string
	Time  time.Time
	Feeds []*block.FeedVO
}

// --- Factory code block ---

type Factory component.Factory[Rule, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Rule, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (Rule, error) {
				m := &mockRule{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Rule, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Rule, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	switch config.EveryDay {
	case "":
		return newWatch(instance, config, dependencies)
	default:
		return newPeriodic(instance, config, dependencies)
	}
}

// --- Implementation code block ---
type mockRule struct {
	component.Mock
}

func (m *mockRule) Config() *Config {
	args := m.Called()

	return args.Get(0).(*Config)
}
