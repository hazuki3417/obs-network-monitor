//go:build windows

package probe

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
	ipSuccess       = 0
	icmpPayloadSize = 32
	icmpReplySize   = 128
)

var (
	iphlpapi            = windows.NewLazySystemDLL("iphlpapi.dll")
	icmpCreateFileProc  = iphlpapi.NewProc("IcmpCreateFile")
	icmpSendEchoProc    = iphlpapi.NewProc("IcmpSendEcho")
	icmpCloseHandleProc = iphlpapi.NewProc("IcmpCloseHandle")
)

type systemICMPProbe struct {
	resolver *net.Resolver
}

type nativeICMPResult struct {
	rtt time.Duration
	err error
}

func newSystemICMPProbe() Runner {
	return &systemICMPProbe{resolver: net.DefaultResolver}
}

func (probe *systemICMPProbe) Probe(ctx context.Context, target string, timeout time.Duration) (time.Duration, error) {
	if timeout <= 0 {
		return 0, errors.New("timeout must be positive")
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	destination, err := probe.resolveIPv4(probeCtx, target)
	if err != nil {
		return 0, err
	}

	nativeTimeout := timeout
	if deadline, ok := probeCtx.Deadline(); ok {
		nativeTimeout = time.Until(deadline)
	}
	if nativeTimeout <= 0 {
		return 0, context.DeadlineExceeded
	}

	result := make(chan nativeICMPResult, 1)
	go func() {
		rtt, err := sendICMPEcho(destination, nativeTimeout)
		result <- nativeICMPResult{rtt: rtt, err: err}
	}()

	select {
	case <-probeCtx.Done():
		return 0, probeCtx.Err()
	case completed := <-result:
		return completed.rtt, completed.err
	}
}

func (probe *systemICMPProbe) resolveIPv4(ctx context.Context, target string) (net.IP, error) {
	if parsed := net.ParseIP(target); parsed != nil {
		if ipv4 := parsed.To4(); ipv4 != nil {
			return ipv4, nil
		}
		return nil, errors.New("IPv6 is not supported")
	}

	addresses, err := probe.resolver.LookupIP(ctx, "ip4", target)
	if err != nil {
		return nil, fmt.Errorf("resolve ICMP target: %w", err)
	}
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4, nil
		}
	}
	return nil, errors.New("ICMP target did not resolve to IPv4")
}

func sendICMPEcho(destination net.IP, timeout time.Duration) (time.Duration, error) {
	handle, _, callErr := icmpCreateFileProc.Call()
	if windows.Handle(handle) == windows.InvalidHandle {
		return 0, windowsCallError(callErr, "IcmpCreateFile failed")
	}
	defer icmpCloseHandleProc.Call(handle)

	payload := [icmpPayloadSize]byte{}
	reply := [icmpReplySize]byte{}
	timeoutMillis := timeout.Milliseconds()
	if timeoutMillis < 1 {
		timeoutMillis = 1
	}
	if timeoutMillis > int64(^uint32(0)) {
		timeoutMillis = int64(^uint32(0))
	}

	address := binary.LittleEndian.Uint32(destination.To4())
	replies, _, callErr := icmpSendEchoProc.Call(
		handle,
		uintptr(address),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(uint32(timeoutMillis)),
	)
	runtime.KeepAlive(payload)
	runtime.KeepAlive(reply)

	if replies == 0 {
		return 0, windowsCallError(callErr, "IcmpSendEcho returned no reply")
	}

	status := binary.LittleEndian.Uint32(reply[4:8])
	if status != ipSuccess {
		return 0, fmt.Errorf("ICMP reply status %d", status)
	}
	rttMillis := binary.LittleEndian.Uint32(reply[8:12])
	return time.Duration(rttMillis) * time.Millisecond, nil
}

func windowsCallError(err error, fallback string) error {
	if err == nil {
		return errors.New(fallback)
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return errors.New(fallback)
	}
	return err
}
