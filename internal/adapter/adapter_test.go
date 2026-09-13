package adapter

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

type fakeResolver struct {
	addresses []net.IP
	err       error
	calls     int
}

func (resolver *fakeResolver) LookupIP(_ context.Context, network, _ string) ([]net.IP, error) {
	resolver.calls++
	if network != "ip4" {
		return nil, errors.New("unexpected network")
	}
	return resolver.addresses, resolver.err
}

type fakePlatformAPI struct {
	bestInterface func(net.IP) (uint32, error)
	interfaceInfo func(uint32) (rawInfo, error)
}

func (api fakePlatformAPI) BestInterface(destination net.IP) (uint32, error) {
	return api.bestInterface(destination)
}

func (api fakePlatformAPI) Interface(index uint32) (rawInfo, error) {
	return api.interfaceInfo(index)
}

func TestInspectReturnsRouteSelectedAdapter(t *testing.T) {
	resolver := &fakeResolver{}
	api := fakePlatformAPI{
		bestInterface: func(destination net.IP) (uint32, error) {
			if !destination.Equal(net.IPv4(8, 8, 8, 8)) {
				t.Fatalf("destination = %v", destination)
			}
			return 12, nil
		},
		interfaceInfo: func(index uint32) (rawInfo, error) {
			if index != 12 {
				t.Fatalf("index = %d, want 12", index)
			}
			return rawInfo{
				Name:                 "Ethernet",
				Description:          "Test adapter",
				InterfaceIndex:       12,
				InterfaceLUID:        99,
				OperStatus:           operStatusUp,
				MediaConnectState:    mediaStateConnected,
				TransmitLinkSpeedBPS: 1_000_000_000,
				ReceiveLinkSpeedBPS:  1_000_000_000,
				TransmitOctets:       1_200,
				ReceiveOctets:        3_400,
			}, nil
		},
	}

	got, err := newInspector(resolver, api).Inspect(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	want := Info{
		Name:                 "Ethernet",
		Description:          "Test adapter",
		InterfaceIndex:       12,
		InterfaceLUID:        99,
		State:                StateConnected,
		TransmitLinkSpeedBPS: 1_000_000_000,
		ReceiveLinkSpeedBPS:  1_000_000_000,
		TransmitOctets:       1_200,
		ReceiveOctets:        3_400,
	}
	if got != want {
		t.Fatalf("Inspect() = %#v, want %#v", got, want)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver calls = %d, want 0", resolver.calls)
	}
}

func TestInspectResolvesHostname(t *testing.T) {
	resolver := &fakeResolver{addresses: []net.IP{net.IPv4(8, 8, 4, 4)}}
	api := fakePlatformAPI{
		bestInterface: func(destination net.IP) (uint32, error) {
			if !destination.Equal(net.IPv4(8, 8, 4, 4)) {
				t.Fatalf("destination = %v", destination)
			}
			return 7, nil
		},
		interfaceInfo: func(uint32) (rawInfo, error) {
			return rawInfo{InterfaceIndex: 7, OperStatus: operStatusUp}, nil
		},
	}

	got, err := newInspector(resolver, api).Inspect(context.Background(), "dns.google")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.InterfaceIndex != 7 {
		t.Fatalf("InterfaceIndex = %d, want 7", got.InterfaceIndex)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", resolver.calls)
	}
}

func TestInspectReevaluatesRouteOnEveryCall(t *testing.T) {
	indexes := []uint32{4, 9}
	calls := 0
	api := fakePlatformAPI{
		bestInterface: func(net.IP) (uint32, error) {
			index := indexes[calls]
			calls++
			return index, nil
		},
		interfaceInfo: func(index uint32) (rawInfo, error) {
			return rawInfo{InterfaceIndex: index, OperStatus: operStatusUp}, nil
		},
	}
	inspector := newInspector(&fakeResolver{}, api)

	first, err := inspector.Inspect(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("first Inspect() error = %v", err)
	}
	second, err := inspector.Inspect(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("second Inspect() error = %v", err)
	}
	if first.InterfaceIndex != 4 || second.InterfaceIndex != 9 {
		t.Fatalf("indexes = %d, %d; want 4, 9", first.InterfaceIndex, second.InterfaceIndex)
	}
}

func TestInspectReturnsDisconnectedWhenRouteIsUnavailable(t *testing.T) {
	api := fakePlatformAPI{
		bestInterface: func(net.IP) (uint32, error) {
			return 0, errors.New("no route")
		},
		interfaceInfo: func(uint32) (rawInfo, error) {
			t.Fatal("Interface() must not be called")
			return rawInfo{}, nil
		},
	}

	got, err := newInspector(&fakeResolver{}, api).Inspect(context.Background(), "8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("Inspect() error = %v, want no route", err)
	}
	if got.State != StateDisconnected {
		t.Fatalf("State = %q, want %q", got.State, StateDisconnected)
	}
}

func TestInspectReturnsIndexWhenInterfaceLookupFails(t *testing.T) {
	api := fakePlatformAPI{
		bestInterface: func(net.IP) (uint32, error) { return 42, nil },
		interfaceInfo: func(uint32) (rawInfo, error) {
			return rawInfo{}, errors.New("adapter disappeared")
		},
	}

	got, err := newInspector(&fakeResolver{}, api).Inspect(context.Background(), "8.8.8.8")
	if err == nil || !strings.Contains(err.Error(), "adapter disappeared") {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.State != StateDisconnected || got.InterfaceIndex != 42 {
		t.Fatalf("Inspect() = %#v", got)
	}
}

func TestInspectReportsTargetResolutionFailure(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("DNS unavailable")}
	api := fakePlatformAPI{
		bestInterface: func(net.IP) (uint32, error) {
			t.Fatal("BestInterface() must not be called")
			return 0, nil
		},
		interfaceInfo: func(uint32) (rawInfo, error) {
			t.Fatal("Interface() must not be called")
			return rawInfo{}, nil
		},
	}

	got, err := newInspector(resolver, api).Inspect(context.Background(), "dns.example")
	if err == nil || !strings.Contains(err.Error(), "DNS unavailable") {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.State != StateDisconnected {
		t.Fatalf("State = %q, want %q", got.State, StateDisconnected)
	}
}

func TestInspectRejectsIPv6(t *testing.T) {
	got, err := newInspector(&fakeResolver{}, fakePlatformAPI{}).Inspect(context.Background(), "2001:db8::1")
	if err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.State != StateDisconnected {
		t.Fatalf("State = %q, want %q", got.State, StateDisconnected)
	}
}

func TestInspectHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, err := newInspector(&fakeResolver{}, fakePlatformAPI{}).Inspect(ctx, "8.8.8.8")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Inspect() error = %v, want context.Canceled", err)
	}
	if got.State != StateDisconnected {
		t.Fatalf("State = %q, want %q", got.State, StateDisconnected)
	}
}

func TestConnectionState(t *testing.T) {
	tests := []struct {
		name  string
		oper  uint32
		media uint32
		want  State
	}{
		{name: "connected", oper: operStatusUp, media: mediaStateConnected, want: StateConnected},
		{name: "up with unknown media", oper: operStatusUp, media: 0, want: StateConnected},
		{name: "media disconnected", oper: operStatusUp, media: mediaStateDisconnected, want: StateDisconnected},
		{name: "oper down", oper: operStatusDown, media: mediaStateConnected, want: StateDisconnected},
		{name: "not present", oper: operStatusNotPresent, media: 0, want: StateDisconnected},
		{name: "lower layer down", oper: operStatusLowerLayerDown, media: 0, want: StateDisconnected},
		{name: "unknown", oper: 4, media: 0, want: StateUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := connectionState(test.oper, test.media); got != test.want {
				t.Fatalf("connectionState() = %q, want %q", got, test.want)
			}
		})
	}
}
