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

package channel

import (
	"context"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/notify/route"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Channel interface {
	component.Component
	sender
}

type sender interface {
	Send(ctx context.Context, receiver Receiver, group *route.FeedGroup) error
}

type Config struct {
	Email *Email
}

func (c *Config) Validate() error {
	if c.Email.Enabled() {
		if err := c.Email.Validate(); err != nil {
			return errors.Wrap(err, "validate email")
		}
	}

	return nil
}

type Receiver struct {
	Email   string
	Webhook *WebhookReceiver
}

func (r *Receiver) Validate() error {
	if r.Email != "" && r.Webhook != nil {
		return errors.New("email and webhook cannot both be set")
	}
	if r.Webhook != nil {
		if err := r.Webhook.Validate(); err != nil {
			return errors.Wrap(err, "validate webhook")
		}
	}

	return nil
}

type Dependencies struct{}

// --- Factory code block ---
type Factory component.Factory[Channel, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Channel, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (Channel, error) {
				m := &mockChannel{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Channel, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Channel, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	var email sender
	if config.Email.Enabled() {
		var err error
		email, err = newEmail(config.Email, dependencies)
		if err != nil {
			return nil, errors.Wrap(err, "new email")
		}
	}

	return &aggrChannel{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "NotifyChannel",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		email:   email,
		webhook: newWebhook(),
	}, nil
}

// --- Implementation code block ---
type aggrChannel struct {
	*component.Base[Config, Dependencies]
	email, webhook sender
}

func (c *aggrChannel) Send(ctx context.Context, receiver Receiver, group *route.FeedGroup) error {
	if receiver.Email != "" && c.email != nil {
		return c.send(ctx, receiver, group, c.email, "email")
	}
	// if receiver.Webhook != nil && c.webhook != nil {
	// TODO: temporarily disable webhook to reduce copyright risks.
	// return c.send(ctx, receiver, group, c.webhook, "webhook")
	// }
	return nil
}

func (c *aggrChannel) send(
	ctx context.Context,
	receiver Receiver,
	group *route.FeedGroup,
	sender sender,
	senderName string,
) (err error) {
	ctx = telemetry.StartWith(ctx, append(c.TelemetryLabels(), telemetrymodel.KeyOperation, "channel", senderName)...)
	defer func() { telemetry.End(ctx, err) }()
	if err := sender.Send(ctx, receiver, group); err != nil {
		return errors.Wrap(err, "send")
	}

	return nil
}

type mockChannel struct {
	component.Mock
}

func (m *mockChannel) Send(ctx context.Context, receiver Receiver, group *route.FeedGroup) error {
	args := m.Called(ctx, receiver, group)

	return args.Error(0)
}
