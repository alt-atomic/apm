package lib

import (
	"fmt"
	"github.com/godbus/dbus/v5"
)

// DBUSConn – глобальное соединение с DBus.
var DBUSConn *dbus.Conn

// InitDBus устанавливает соединение с сессионной шиной DBus
func InitDBus(isSystem bool) error {
	var err error
	if isSystem {
		DBUSConn, err = dbus.ConnectSystemBus()
	} else {
		DBUSConn, err = dbus.ConnectSessionBus()
	}
	if err != nil {
		return err
	}

	reply, err := DBUSConn.RequestName("com.application.APM", dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}

	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("интерфейс com.application.APM уже занят")
	}

	return nil
}
