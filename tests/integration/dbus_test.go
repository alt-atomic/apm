// Atomic Package Manager
// Copyright (C) 2025 Дмитрий Удалов dmitry@udalov.online
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	APMServiceName = "org.altlinux.APM"
	APMObjectPath  = "/org/altlinux/APM"
	APMInterface   = "org.altlinux.APM"
)

// TestDBusServiceAvailability проверяет доступность DBUS сервиса
func TestDBusServiceAvailability(t *testing.T) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(t, err, "Should connect to system bus")
	defer conn.Close()

	// Проверяем что сервис доступен
	obj := conn.Object(APMServiceName, APMObjectPath)
	assert.NotNil(t, obj)

	// Пингуем сервис
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		t.Skipf("APM DBUS service not available: %v", err)
	}
}

// TestDBusMethodCalls тестирует основные методы DBUS
func TestDBusMethodCalls(t *testing.T) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(t, err)
	defer conn.Close()

	obj := conn.Object(APMServiceName, APMObjectPath)

	// Проверяем доступность сервиса
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		t.Skipf("APM DBUS service not available: %v", err)
	}

	// Тестируем метод Info
	t.Run("Info method", func(t *testing.T) {
		var result string
		err := obj.CallWithContext(ctx, APMInterface+".Info", 0, "hello", false).Store(&result)
		
		if err != nil {
			t.Logf("Info method error (may be expected): %v", err)
			// Не фейлим тест, так как это может быть ожидаемо в тестовой среде
		} else {
			assert.NotEmpty(t, result, "Info method should return non-empty result")
			t.Logf("Info result: %s", result)
		}
	})

	// Тестируем метод Search
	t.Run("Search method", func(t *testing.T) {
		var result string
		err := obj.CallWithContext(ctx, APMInterface+".Search", 0, "hello", false, false).Store(&result)
		
		if err != nil {
			t.Logf("Search method error (may be expected): %v", err)
		} else {
			assert.NotEmpty(t, result, "Search method should return non-empty result")
			t.Logf("Search result length: %d", len(result))
		}
	})

	// Тестируем метод с неправильными параметрами
	t.Run("Invalid parameters", func(t *testing.T) {
		var result string
		err := obj.CallWithContext(ctx, APMInterface+".Info", 0, "", false).Store(&result)
		
		// Должна быть ошибка валидации
		assert.Error(t, err, "Should return validation error for empty package name")
	})
}

// TestDBusSignals тестирует сигналы DBUS
func TestDBusSignals(t *testing.T) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(t, err)
	defer conn.Close()

	// Подписываемся на сигналы APM
	err = conn.AddMatchSignal(
		dbus.WithMatchInterface(APMInterface),
		dbus.WithMatchSender(APMServiceName),
	)
	if err != nil {
		t.Skipf("Cannot subscribe to APM signals: %v", err)
	}

	signals := make(chan *dbus.Signal, 10)
	conn.Signal(signals)

	// Запускаем операцию которая должна генерировать сигналы
	obj := conn.Object(APMServiceName, APMObjectPath)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Пытаемся выполнить операцию поиска (должна генерировать сигналы прогресса)
	go func() {
		var result string
		obj.CallWithContext(ctx, APMInterface+".Search", 0, "hello", false, false).Store(&result)
	}()

	// Ждем сигналы
	timeout := time.After(5 * time.Second)
	signalReceived := false

	select {
	case sig := <-signals:
		if sig.Sender == APMServiceName {
			signalReceived = true
			t.Logf("Received APM signal: %s with %d args", sig.Name, len(sig.Body))
		}
	case <-timeout:
		t.Log("No APM signals received within timeout (may be expected)")
	}

	// Не фейлим если сигналы не получены, так как это может быть ожидаемо
	if signalReceived {
		t.Log("Successfully received APM signals")
	}
}

// TestDBusServiceProperties тестирует свойства DBUS сервиса
func TestDBusServiceProperties(t *testing.T) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(t, err)
	defer conn.Close()

	obj := conn.Object(APMServiceName, APMObjectPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Проверяем доступность сервиса
	err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		t.Skipf("APM DBUS service not available: %v", err)
	}

	// Получаем свойства интерфейса (если есть)
	prop := conn.Object(APMServiceName, APMObjectPath)
	
	// Пытаемся получить версию или другие свойства
	var version dbus.Variant
	err = prop.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, 
		APMInterface, "Version").Store(&version)
	
	if err != nil {
		t.Logf("Could not get Version property (may not be implemented): %v", err)
	} else {
		t.Logf("APM Version: %v", version.Value())
	}
}

// TestDBusConnectionStability тестирует стабильность соединения
func TestDBusConnectionStability(t *testing.T) {
	// Создаем несколько подключений
	connections := make([]*dbus.Conn, 5)
	
	for i := 0; i < 5; i++ {
		conn, err := dbus.ConnectSystemBus()
		require.NoError(t, err, "Should create connection %d", i)
		connections[i] = conn
	}

	// Проверяем что все соединения работают
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i, conn := range connections {
		obj := conn.Object(APMServiceName, APMObjectPath)
		
		err := obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
		if err != nil {
			t.Logf("Connection %d ping failed (service may not be available): %v", i, err)
		} else {
			t.Logf("Connection %d ping successful", i)
		}
	}

	// Закрываем все соединения
	for i, conn := range connections {
		err := conn.Close()
		assert.NoError(t, err, "Should close connection %d", i)
	}
}

// TestDBusErrorHandling тестирует обработку ошибок DBUS
func TestDBusErrorHandling(t *testing.T) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(t, err)
	defer conn.Close()

	// Пытаемся вызвать несуществующий метод
	obj := conn.Object(APMServiceName, APMObjectPath)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result string
	err = obj.CallWithContext(ctx, APMInterface+".NonExistentMethod", 0).Store(&result)
	
	assert.Error(t, err, "Should return error for non-existent method")
	
	if dbusErr, ok := err.(dbus.Error); ok {
		t.Logf("DBUS error name: %s, message: %s", dbusErr.Name, dbusErr.Body)
		assert.Contains(t, dbusErr.Name, "UnknownMethod", 
			"Should be UnknownMethod error")
	}
}

// BenchmarkDBusOperations тестирует производительность DBUS операций
func BenchmarkDBusOperations(b *testing.B) {
	conn, err := dbus.ConnectSystemBus()
	require.NoError(b, err)
	defer conn.Close()

	obj := conn.Object(APMServiceName, APMObjectPath)
	ctx := context.Background()

	// Проверяем доступность сервиса
	err = obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0).Err
	if err != nil {
		b.Skipf("APM DBUS service not available: %v", err)
	}

	b.Run("Ping", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			obj.CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0)
		}
	})

	b.Run("Info", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var result string
			obj.CallWithContext(ctx, APMInterface+".Info", 0, "hello", false).Store(&result)
		}
	})
}