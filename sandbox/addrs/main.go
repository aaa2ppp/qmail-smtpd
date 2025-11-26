package main

import (
	"fmt"
	"net"
)

type InterfaceInfo struct {
	Name  string
	MAC   string
	Flags string
	IPv4  []string
	IPv6  []string
}

func getDetailedInterfaceInfo() []InterfaceInfo {
	var interfacesInfo []InterfaceInfo

	interfaces, err := net.Interfaces()
	if err != nil {
		return interfacesInfo
	}

	for _, iface := range interfaces {
		info := InterfaceInfo{
			Name:  iface.Name,
			MAC:   iface.HardwareAddr.String(),
			Flags: iface.Flags.String(),
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			if ip.To4() != nil {
				info.IPv4 = append(info.IPv4, ip.String())
			} else {
				info.IPv6 = append(info.IPv6, ip.String())
			}
		}

		interfacesInfo = append(interfacesInfo, info)
	}

	return interfacesInfo
}

func main() {
	interfacesInfo := getDetailedInterfaceInfo()

	fmt.Println("Детальная информация о сетевых интерфейсах:")
	fmt.Println("===========================================")

	for _, info := range interfacesInfo {
		fmt.Printf("Интерфейс: %s\n", info.Name)
		fmt.Printf("  MAC: %s\n", info.MAC)
		fmt.Printf("  Флаги: %s\n", info.Flags)

		if len(info.IPv4) > 0 {
			fmt.Printf("  IPv4 адреса:\n")
			for _, ip := range info.IPv4 {
				fmt.Printf("    - %s\n", ip)
			}
		}

		if len(info.IPv6) > 0 {
			fmt.Printf("  IPv6 адреса:\n")
			for _, ip := range info.IPv6 {
				fmt.Printf("    - %s\n", ip)
			}
		}
		fmt.Println()
	}
}
