package models

import (
	"context"

	"github.com/edouardparis/lntop/app"
	"github.com/edouardparis/lntop/logging"
	"github.com/edouardparis/lntop/network"
	"github.com/edouardparis/lntop/network/models"
	"github.com/edouardparis/lntop/network/options"
)

type Models struct {
	logger          logging.Logger
	network         *network.Network
	Info            *Info
	Channels        *Channels
	WalletBalance   *WalletBalance
	ChannelsBalance *ChannelsBalance
	Transactions    *Transactions
	RoutingLog      *RoutingLog
}

func New(app *app.App) *Models {
	return &Models{
		logger:          app.Logger.With(logging.String("logger", "models")),
		network:         app.Network,
		Info:            &Info{},
		Channels:        NewChannels(),
		WalletBalance:   &WalletBalance{},
		ChannelsBalance: &ChannelsBalance{},
		Transactions:    &Transactions{},
		RoutingLog:      &RoutingLog{},
	}
}

type Info struct {
	*models.Info
}

func (m *Models) RefreshInfo(ctx context.Context) error {
	info, err := m.network.Info(ctx)
	if err != nil {
		return err
	}
	*m.Info = Info{info}
	return nil
}

func (m *Models) RefreshChannels(ctx context.Context) error {
	channels, err := m.network.ListChannels(ctx, options.WithChannelPending)
	if err != nil {
		return err
	}
	for i := range channels {
		if !m.Channels.Contains(channels[i]) {
			m.Channels.Add(channels[i])
		}
		channel := m.Channels.GetByChanPoint(channels[i].ChannelPoint)
		if channel != nil &&
			(channel.UpdatesCount < channels[i].UpdatesCount ||
				channel.LastUpdate == nil) {
			err := m.network.GetChannelInfo(ctx, channels[i])
			if err != nil {
				return err
			}

			if channels[i].Node == nil {
				channels[i].Node, err = m.network.GetNode(ctx,
					channels[i].RemotePubKey)
				if err != nil {
					m.logger.Debug("refreshChannels: cannot find Node",
						logging.String("pubkey", channels[i].RemotePubKey))
				}
			}
		}

		m.Channels.Update(channels[i])
	}
	return nil
}

type WalletBalance struct {
	*models.WalletBalance
}

func (m *Models) RefreshWalletBalance(ctx context.Context) error {
	balance, err := m.network.GetWalletBalance(ctx)
	if err != nil {
		return err
	}
	*m.WalletBalance = WalletBalance{balance}
	return nil
}

type ChannelsBalance struct {
	*models.ChannelsBalance
}

func (m *Models) RefreshChannelsBalance(ctx context.Context) error {
	balance, err := m.network.GetChannelsBalance(ctx)
	if err != nil {
		return err
	}
	*m.ChannelsBalance = ChannelsBalance{balance}
	return nil
}

type RoutingLog struct {
	Log []*models.RoutingEvent
}

const MaxRoutingEvents = 512 // 8K monitor @ 8px per line = 540

func (m *Models) RefreshRouting(update interface{}) func(context.Context) error {
	return (func(ctx context.Context) error {
		hu, ok := update.(*models.RoutingEvent)
		if ok {
			found := false
			for _, hlu := range m.RoutingLog.Log {
				if hlu.Equals(hu) {
					hlu.Update(hu)
					found = true
					break
				}
			}
			if !found {
				if len(m.RoutingLog.Log) == MaxRoutingEvents {
					m.RoutingLog.Log = m.RoutingLog.Log[1:]
				}
				m.RoutingLog.Log = append(m.RoutingLog.Log, hu)
			}
		} else {
			m.logger.Error("refreshRouting: invalid event data")
		}
		return nil
	})
}
