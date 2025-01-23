//go:build !baremetal

// Some documentation for the BlueZ D-Bus interface:
// https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc

package bluetooth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

const defaultAdapter = "hci0"

type Adapter struct {
	id                   string
	scanCancelChan       chan struct{}
	bus                  *dbus.Conn
	bluez                dbus.BusObject    // object at /
	adapter              dbus.BusObject    // object at /org/bluez/hciX
	signal               chan *dbus.Signal // Signal handler
	address              string
	defaultAdvertisement *Advertisement

	connectHandler func(device Device, connected bool)
}

// NewAdapter creates a new Adapter with the given ID.
//
// Make sure to call Enable() before using it to initialize the adapter.
func NewAdapter(id string) *Adapter {
	return &Adapter{
		id:             id,
		connectHandler: func(device Device, connected bool) {},
	}
}

// DefaultAdapter is the default adapter on the system. On Linux, it is the
// first adapter available.
//
// Make sure to call Enable() before using it to initialize the adapter.
var DefaultAdapter = NewAdapter(defaultAdapter)

// Enable configures the BLE stack. It must be called before any
// Bluetooth-related calls (unless otherwise indicated).
func (a *Adapter) Enable() (err error) {
	bus, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	a.bus = bus
	a.bluez = a.bus.Object("org.bluez", dbus.ObjectPath("/"))
	a.adapter = a.bus.Object("org.bluez", dbus.ObjectPath("/org/bluez/"+a.id))
	addr, err := a.adapter.GetProperty("org.bluez.Adapter1.Address")
	if err != nil {
		if err, ok := err.(dbus.Error); ok && err.Name == "org.freedesktop.DBus.Error.UnknownObject" {
			return fmt.Errorf("bluetooth: adapter %s does not exist", a.adapter.Path())
		}
		return fmt.Errorf("could not activate BlueZ adapter: %w", err)
	}
	addr.Store(&a.address)

	// Attach connect/disconnect handler
	// Similar to how bluetoothctl does it, and more clearly how bluez/test/monitor-bluetooth does it
	// Our signal body looks like this:
	// &{:1.8 Sender
	// /org/bluez/hci0/dev_DC_E5_5B_13_10_54 Path
	// org.freedesktop.DBus.Properties.PropertiesChanged Name
	// [org.bluez.Device1 map[Connected:false ServicesResolved:false] []] Body
	// 23 Sequence}
	err = a.bus.AddMatchSignal(
		dbus.WithMatchSender("org.bluez"),                          // Bus name
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"), // Interface name
	)

	if err != nil {
		return fmt.Errorf("Failed to attach connection listener")
	}

	// Make sure signal is initialized
	if a.signal == nil {
		a.signal = make(chan *dbus.Signal)
		a.bus.Signal(a.signal)
		go func() {
			// DBus sends us pretty much all signals, here they are filtered
			// for ones that actually interest us (device connectivity, etc...)
			for sig := range a.signal {
				//fmt.Printf("%v\n", sig)
				if a.connectHandler != nil {
					interfaceName := sig.Body[0].(string)
					if interfaceName != "org.bluez.Device1" {
						continue // Only listen to bluez device signals
					}
					if sig.Name != "org.freedesktop.DBus.Properties.PropertiesChanged" {
						continue // Only property changes
					}
					// TODO test if for this adapter

					// Get signal body, usually includes "Connected" and "ServicesResolved"
					changes := sig.Body[1].(map[string]dbus.Variant)
					isConnected := changes["Connected"]
					// REVIEW mount doom...
					if !isConnected.Signature().Empty() {
						device := a.bus.Object("org.bluez", sig.Path)
						devicePath, _ := MakeAddress(strings.Split(string(sig.Path), "dev_")[1])
						a.connectHandler(Device{Address: devicePath, device: device, adapter: a},
							isConnected.Value().(bool))
					}
				}
			}
		}()
	}
	return nil
}

func (a *Adapter) Disable() {
	close(a.signal)
	a.bus.RemoveSignal(a.signal)
	a.signal = nil
	//a.bus.RemoveMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus.Properties"))
}

func (a *Adapter) Address() (MACAddress, error) {
	if a.address == "" {
		return MACAddress{}, errors.New("adapter not enabled")
	}
	mac, err := ParseMAC(a.address)
	if err != nil {
		return MACAddress{}, err
	}
	return MACAddress{MAC: mac}, nil
}
