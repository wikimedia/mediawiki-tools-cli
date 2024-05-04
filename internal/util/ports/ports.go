package ports

import (
	"errors"
	"net"
	"strconv"
)

var (
	defaultStartingPort        = "8080"
	portSearchLoopsBeforePanic = 25
)

/*FreeUpFrom get a free port up from and including the startingPort. Will attempt 25 ports.*/
func FreeUpFrom(startingPort string) string {
	if isValid(startingPort) != nil {
		startingPort = defaultStartingPort
	}

	portFloat, _ := strconv.ParseFloat(startingPort, 64)
	loops := 1

	for portFloat < 65535 && loops <= portSearchLoopsBeforePanic {
		loopPort := strconv.FormatFloat(portFloat, 'f', 0, 64)
		if IsValidAndFree(loopPort) == nil {
			return loopPort
		}
		portFloat++
		loops++
	}

	panic("Error: Too many loops finding free port")
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
		return errors.New("port is not available to listen on")
	}

	closeErr := ln.Close()
	if closeErr != nil {
		return closeErr
	}
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
