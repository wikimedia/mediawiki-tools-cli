/*Package ports in internal utils is functionality for interacting with ports

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
package ports

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
)

/*FreeUpFrom get a free port up from and including the startingPort. Will attempt 25 ports.*/
func FreeUpFrom(startingPort string) string {
	if isValid(startingPort) != nil {
		startingPort = "8080"
	}

	portFloat, _ := strconv.ParseFloat(startingPort, 64)
	loops := 1

	for portFloat < 65535 && loops <= 25 {
		loopPort := strconv.FormatFloat(portFloat, 'f', 0, 64)
		if IsValidAndFree(loopPort) == nil {
			return loopPort
		}
		portFloat++
		loops++
	}

	log.Fatal(errors.New("too many loops finding free port"))
	os.Exit(1)
	return ""
}

/*IsValidAndFree ...*/
func IsValidAndFree(port string) error {
	err := isValid(port)
	if err != nil {
		return err
	}
	err = isFree(port)
	if err != nil {
		return err
	}
	return nil
}

func isFree(port string) error {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return errors.New("port is not availible to listen on")
	}

	ln.Close()
	return nil
}

func isValid(port string) error {
	parsedPort, err := strconv.ParseFloat(port, 64)
	if err != nil {
		return errors.New("invalid number")
	}
	if parsedPort > 65535 || parsedPort < 1 {
		return errors.New("invalid port number")
	}
	return nil
}
