package adapter

import (
	"context"
	"errors"
	"fmt"
	"net"
)

type State string

const (
	StateConnected    State = "connected"
	StateDisconnected State = "disconnected"
	StateUnknown      State = "unknown"
)

const (
	operStatusUp             = 1
	operStatusDown           = 2
	operStatusNotPresent     = 6
	operStatusLowerLayerDown = 7
	mediaStateConnected      = 1
	mediaStateDisconnected   = 2
)

type Info struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	InterfaceIndex       uint32 `json:"interfaceIndex"`
	InterfaceLUID        uint64 `json:"interfaceLuid"`
	State                State  `json:"state"`
	TransmitLinkSpeedBPS uint64 `json:"transmitLinkSpeedBps"`
	ReceiveLinkSpeedBPS  uint64 `json:"receiveLinkSpeedBps"`
}

type Provider interface {
	Inspect(ctx context.Context, target string) (Info, error)
}

type ipResolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

type platformAPI interface {
	BestInterface(destination net.IP) (uint32, error)
	Interface(index uint32) (rawInfo, error)
}

type rawInfo struct {
	Name                 string
	Description          string
	InterfaceIndex       uint32
	InterfaceLUID        uint64
	OperStatus           uint32
	MediaConnectState    uint32
	TransmitLinkSpeedBPS uint64
	ReceiveLinkSpeedBPS  uint64
}

type Inspector struct {
	resolver ipResolver
	api      platformAPI
}

func NewInspector() *Inspector {
	return newInspector(net.DefaultResolver, newPlatformAPI())
}

func newInspector(resolver ipResolver, api platformAPI) *Inspector {
	return &Inspector{resolver: resolver, api: api}
}

func (inspector *Inspector) Inspect(ctx context.Context, target string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return disconnected(), err
	}

	destination, err := resolveIPv4(ctx, inspector.resolver, target)
	if err != nil {
		return disconnected(), fmt.Errorf("resolve adapter target %q: %w", target, err)
	}
	if err := ctx.Err(); err != nil {
		return disconnected(), err
	}

	index, err := inspector.api.BestInterface(destination)
	if err != nil {
		return disconnected(), fmt.Errorf("find best interface for %s: %w", destination, err)
	}

	row, err := inspector.api.Interface(index)
	if err != nil {
		result := disconnected()
		result.InterfaceIndex = index
		return result, fmt.Errorf("get interface %d: %w", index, err)
	}

	return Info{
		Name:                 row.Name,
		Description:          row.Description,
		InterfaceIndex:       row.InterfaceIndex,
		InterfaceLUID:        row.InterfaceLUID,
		State:                connectionState(row.OperStatus, row.MediaConnectState),
		TransmitLinkSpeedBPS: row.TransmitLinkSpeedBPS,
		ReceiveLinkSpeedBPS:  row.ReceiveLinkSpeedBPS,
	}, nil
}

func resolveIPv4(ctx context.Context, resolver ipResolver, target string) (net.IP, error) {
	if parsed := net.ParseIP(target); parsed != nil {
		if ipv4 := parsed.To4(); ipv4 != nil {
			return ipv4, nil
		}
		return nil, errors.New("IPv6 is not supported")
	}

	addresses, err := resolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}
	return nil, errors.New("target did not resolve to an IPv4 address")
}

func connectionState(operStatus, mediaConnectState uint32) State {
	if mediaConnectState == mediaStateDisconnected ||
		operStatus == operStatusDown ||
		operStatus == operStatusNotPresent ||
		operStatus == operStatusLowerLayerDown {
		return StateDisconnected
	}
	if operStatus == operStatusUp && mediaConnectState != mediaStateDisconnected {
		return StateConnected
	}
	return StateUnknown
}

func disconnected() Info {
	return Info{State: StateDisconnected}
}
