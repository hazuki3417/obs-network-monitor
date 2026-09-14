//go:build windows

package traceroute

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ipSuccess           = 0
	ipRequestTimedOut   = 11010
	ipTTLExpiredTransit = 11013
	icmpPayloadSize     = 32
	icmpReplySize       = 128
)

var (
	tracerouteIPHLPAPI        = windows.NewLazySystemDLL("iphlpapi.dll")
	tracerouteCreateFileProc  = tracerouteIPHLPAPI.NewProc("IcmpCreateFile")
	tracerouteSendEchoProc    = tracerouteIPHLPAPI.NewProc("IcmpSendEcho")
	tracerouteCloseHandleProc = tracerouteIPHLPAPI.NewProc("IcmpCloseHandle")
)

type ipOptionInformation struct {
	TTL         byte
	TOS         byte
	Flags       byte
	OptionsSize byte
	OptionsData *byte
}

type systemHopRunner struct {
	resolver    *net.Resolver
	target      string
	destination net.IP
}

type nativeHopResult struct {
	response HopResponse
	err      error
}

func newSystemHopRunner() HopRunner {
	return &systemHopRunner{resolver: net.DefaultResolver}
}

func (runner *systemHopRunner) Probe(
	ctx context.Context,
	target string,
	ttl int,
	timeout time.Duration,
) (HopResponse, error) {
	if ttl < 1 || ttl > 255 {
		return HopResponse{}, errors.New("TTL must be between 1 and 255")
	}
	if timeout <= 0 {
		return HopResponse{}, errors.New("timeout must be positive")
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	destination, err := runner.resolveIPv4(probeCtx, target)
	if err != nil {
		return HopResponse{}, err
	}
	nativeTimeout := timeout
	if deadline, ok := probeCtx.Deadline(); ok {
		nativeTimeout = time.Until(deadline)
	}
	if nativeTimeout <= 0 {
		return HopResponse{}, context.DeadlineExceeded
	}

	completed := make(chan nativeHopResult, 1)
	go func() {
		response, probeErr := sendHop(destination, ttl, nativeTimeout)
		completed <- nativeHopResult{response: response, err: probeErr}
	}()

	select {
	case <-probeCtx.Done():
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return HopResponse{}, nil
		}
		return HopResponse{}, probeCtx.Err()
	case result := <-completed:
		return result.response, result.err
	}
}

func (runner *systemHopRunner) resolveIPv4(ctx context.Context, target string) (net.IP, error) {
	if runner.target == target && runner.destination != nil {
		return append(net.IP(nil), runner.destination...), nil
	}
	if parsed := net.ParseIP(target); parsed != nil {
		if ipv4 := parsed.To4(); ipv4 != nil {
			runner.target = target
			runner.destination = append(net.IP(nil), ipv4...)
			return ipv4, nil
		}
		return nil, errors.New("IPv6 is not supported")
	}
	addresses, err := runner.resolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return nil, fmt.Errorf("resolve traceroute target: %w", err)
	}
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			runner.target = target
			runner.destination = append(net.IP(nil), ipv4...)
			return ipv4, nil
		}
	}
	return nil, errors.New("traceroute target did not resolve to IPv4")
}

func sendHop(destination net.IP, ttl int, timeout time.Duration) (HopResponse, error) {
	handle, _, callErr := tracerouteCreateFileProc.Call()
	if windows.Handle(handle) == windows.InvalidHandle {
		return HopResponse{}, tracerouteWindowsError(callErr, "IcmpCreateFile failed")
	}
	defer tracerouteCloseHandleProc.Call(handle)

	payload := [icmpPayloadSize]byte{}
	reply := [icmpReplySize]byte{}
	options := ipOptionInformation{TTL: byte(ttl)}
	timeoutMillis := timeout.Milliseconds()
	if timeoutMillis < 1 {
		timeoutMillis = 1
	}
	if timeoutMillis > int64(^uint32(0)) {
		timeoutMillis = int64(^uint32(0))
	}

	address := binary.LittleEndian.Uint32(destination.To4())
	replies, _, callErr := tracerouteSendEchoProc.Call(
		handle,
		uintptr(address),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		uintptr(unsafe.Pointer(&options)),
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(uint32(timeoutMillis)),
	)
	runtime.KeepAlive(payload)
	runtime.KeepAlive(reply)
	runtime.KeepAlive(options)

	if replies == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && (errno == 0 || errno == ipRequestTimedOut) {
			return HopResponse{}, nil
		}
		return HopResponse{}, tracerouteWindowsError(callErr, "IcmpSendEcho returned no reply")
	}

	status := binary.LittleEndian.Uint32(reply[4:8])
	if status == ipRequestTimedOut {
		return HopResponse{}, nil
	}
	addressIP := net.IPv4(reply[0], reply[1], reply[2], reply[3]).To4()
	response := HopResponse{
		Address:   addressIP.String(),
		Responded: status == ipSuccess || status == ipTTLExpiredTransit,
		RTT:       time.Duration(binary.LittleEndian.Uint32(reply[8:12])) * time.Millisecond,
		Target:    status == ipSuccess && addressIP.Equal(destination),
	}
	return response, nil
}

func tracerouteWindowsError(err error, fallback string) error {
	if err == nil {
		return errors.New(fallback)
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return errors.New(fallback)
	}
	return err
}
