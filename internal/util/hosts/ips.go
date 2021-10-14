/*Package hosts in internal utils is functionality for interacting with an etc hosts file

Copyright © 2020 Addshore

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
package hosts

import (
	"net"
)

var ipv4AddressOverride = ""

/*IPs ... */
func IPs(host string) ([]string, error) {
	// Resolver https://gist.github.com/aojea/94f6f483173641647c731f582e52f0b0#file-resolve_localhost-go-L11
	addrs, err := net.LookupHost(host)
	//fmt.Println("net.LookupHost addrs:", addrs, "err:", err)
	return addrs, err
}

func addressType(ip string) int {
	if net.ParseIP(ip) == nil {
		// Invalid address type..
		return 0
	}
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			return 4
		case ':':
			return 6
		}
	}
	return -1
}

func getFirstOfType(addrs []string, t int) *string {
	for _, a := range addrs {
		if addressType(a) == t {
			return &a
		}
	}
	return nil
}

/*IPv4 ...*/
func IPv4(host string) string {
	if ipv4AddressOverride != "" {
		return ipv4AddressOverride
	}
	addrs, _ := IPs(host)
	return *getFirstOfType(addrs, 4)
}

/*IPv6 ...*/
func IPv6(host string) string {
	addrs, _ := IPs(host)
	return *getFirstOfType(addrs, 6)
}
