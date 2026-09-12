//go:build !windows

package adapter

import (
	"errors"
	"net"
)

type unsupportedAPI struct{}

func newPlatformAPI() platformAPI {
	return unsupportedAPI{}
}

func (unsupportedAPI) BestInterface(net.IP) (uint32, error) {
	return 0, errors.New("network adapter inspection is only supported on Windows")
}

func (unsupportedAPI) Interface(uint32) (rawInfo, error) {
	return rawInfo{}, errors.New("network adapter inspection is only supported on Windows")
}
