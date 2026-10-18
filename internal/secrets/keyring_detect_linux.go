package secrets

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

func DetectKeyring(ctx context.Context) Availability {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	unavailable := Availability{Detail: "no usable persistent Secret Service keyring detected"}
	// No autostart, Unlock, GetSecret or write calls during discovery.
	conn, err := dbus.SessionBusPrivateNoAutoStartup(dbus.WithContext(ctx))
	if err != nil {
		return unavailable
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.Auth(nil); err != nil {
		return unavailable
	}
	if err := conn.Hello(); err != nil {
		return unavailable
	}
	service := conn.Object("org.freedesktop.secrets", "/org/freedesktop/secrets")
	collection := dbus.ObjectPath("/org/freedesktop/secrets/collection/login")
	locked := func(path dbus.ObjectPath) (bool, error) {
		var property dbus.Variant
		err := conn.Object("org.freedesktop.secrets", path).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", dbus.FlagNoAutoStart, "org.freedesktop.Secret.Collection", "Locked").Store(&property)
		if err != nil {
			return false, err
		}
		var result bool
		if err := property.Store(&result); err != nil {
			return false, err
		}
		return result, nil
	}
	isLocked, err := locked(collection)
	if err != nil {
		if err := service.CallWithContext(ctx, "org.freedesktop.Secret.Service.ReadAlias", dbus.FlagNoAutoStart, "default").Store(&collection); err != nil {
			return unavailable
		}
		if collection == "/" || collection == "/org/freedesktop/secrets/collection/session" {
			return unavailable
		}
		isLocked, err = locked(collection)
		if err != nil {
			return unavailable
		}
	}
	if isLocked {
		return Availability{Available: true, Detail: "persistent Secret Service keyring found; unlocking may be required"}
	}
	return Availability{Available: true, Detail: "persistent Secret Service keyring found"}
}
