// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import (
	"fmt"
	"net"

	current "github.com/containernetworking/cni/pkg/types/040"

	"github.com/cilium/cilium/api/v1/models"
	linuxrouting "github.com/cilium/cilium/pkg/datapath/linux/routing"
	"github.com/cilium/cilium/pkg/ip"
)

func interfaceAdd(ipConfig *current.IPConfig, ipam *models.IPAMAddressResponse, conf models.DaemonConfigurationStatus) error {
	// If the gateway IP is not available, it is already set up
	if ipam.Gateway == "" {
		return nil
	}

	var masq bool
	if ipConfig.Version == "4" {
		masq = conf.MasqueradeProtocols.IPV4
	} else if ipConfig.Version == "6" {
		masq = conf.MasqueradeProtocols.IPV6
	} else {
		return fmt.Errorf("Invalid IPConfig version: %s", ipConfig.Version)
	}

	allCIDRs := make([]*net.IPNet, 0, len(ipam.Cidrs))
	for _, cidrString := range ipam.Cidrs {
		_, cidr, err := net.ParseCIDR(cidrString)
		if err != nil {
			return fmt.Errorf("invalid CIDR '%s': %s", cidrString, err)
		}
		allCIDRs = append(allCIDRs, cidr)
	}
	// Coalesce CIDRs into minimum set needed for route rules
	// The routes set up here will be cleaned up by linuxrouting.Delete.
	// Therefore the code here should be kept in sync with the deletion code.
	ipv4CIDRs, _ := ip.CoalesceCIDRs(allCIDRs)
	cidrs := make([]string, 0, len(ipv4CIDRs))
	for _, cidr := range ipv4CIDRs {
		cidrs = append(cidrs, cidr.String())
	}

	routingInfo, err := linuxrouting.NewRoutingInfo(
		ipam.Gateway,
		cidrs,
		ipam.MasterMac,
		ipam.InterfaceNumber,
		masq,
	)
	if err != nil {
		return fmt.Errorf("unable to parse routing info: %v", err)
	}

	if err := routingInfo.Configure(
		ipConfig.Address.IP,
		int(conf.DeviceMTU),
		conf.EgressMultiHomeIPRuleCompat,
	); err != nil {
		return fmt.Errorf("unable to install ip rules and routes: %s", err)
	}

	return nil
}
