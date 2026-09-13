//go:build windows

package adapter

import (
	"fmt"
	"net"

	"golang.org/x/sys/windows"
)

type windowsAPI struct{}

func newPlatformAPI() platformAPI {
	return windowsAPI{}
}

func (windowsAPI) BestInterface(destination net.IP) (uint32, error) {
	ipv4 := destination.To4()
	if ipv4 == nil {
		return 0, fmt.Errorf("destination %q is not IPv4", destination)
	}

	address := &windows.SockaddrInet4{}
	copy(address.Addr[:], ipv4)

	var index uint32
	if err := windows.GetBestInterfaceEx(address, &index); err != nil {
		return 0, err
	}
	return index, nil
}

func (windowsAPI) Interface(index uint32) (rawInfo, error) {
	row := windows.MibIfRow2{InterfaceIndex: index}
	if err := windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row); err != nil {
		return rawInfo{}, err
	}

	return rawInfo{
		Name:                 windows.UTF16ToString(row.Alias[:]),
		Description:          windows.UTF16ToString(row.Description[:]),
		InterfaceIndex:       row.InterfaceIndex,
		InterfaceLUID:        row.InterfaceLuid,
		OperStatus:           row.OperStatus,
		MediaConnectState:    row.MediaConnectState,
		TransmitLinkSpeedBPS: row.TransmitLinkSpeed,
		ReceiveLinkSpeedBPS:  row.ReceiveLinkSpeed,
		TransmitOctets:       row.OutOctets,
		ReceiveOctets:        row.InOctets,
	}, nil
}
