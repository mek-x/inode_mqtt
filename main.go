package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"
	"encoding/binary"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci/cmd"
	"github.com/pkg/errors"
)

var (
	devAddr = flag.String("a", "d0:f0:18:44:00:0c", "ble device address")
	dup     = flag.Bool("d", false, "allow duplicate reported")
	scanType = flag.Bool("as", false, "acitve scan")
)

func main() {
	flag.Parse()

	scanParams := cmd.LESetScanParameters{
		LEScanType:           0x00,   // 0x00: passive, 0x01: active
		LEScanInterval:       0x0004, // 0x0004 - 0x4000; N * 0.625msec
		LEScanWindow:         0x0004, // 0x0004 - 0x4000; N * 0.625msec
		OwnAddressType:       0x00,   // 0x00: public, 0x01: random
		ScanningFilterPolicy: 0x00,   // 0x00: accept all, 0x01: ignore non-white-listed.
	}

	if *scanType {
		scanParams.LEScanType = 0x01
	}

	d, err := linux.NewDevice(ble.OptScanParams(scanParams))
	if err != nil {
		log.Fatalf("can't get device : %s", err)
	}
	ble.SetDefaultDevice(d)

	// Scan for specified durantion, or until interrupted by user.
	fmt.Printf("Scanning...\n")
	ctx := ble.WithSigHandler(context.WithCancel(context.Background()))
	chkErr(ble.Scan(ctx, *dup, advHandler, nil))
}

func fromBytesToUint16(b []byte) uint16 {
    bits := binary.LittleEndian.Uint16(b)
    return bits
}

func min(x, y uint16) uint16 {
    if x <= y {
        return x
    }
    return y
}

func advHandler(a ble.Advertisement) {
	if a.Addr().String() != *devAddr {
		return
	}

	t := time.Now()
	fmt.Printf("%s: ", t.Format("2006-01-02 15:04:05"))

	fmt.Printf("RSSI = %ddBm", a.RSSI())

	if len(a.ManufacturerData()) > 0 {
		md := a.ManufacturerData()

		rawBattery := fromBytesToUint16(md[2:4])
		battery := (rawBattery >> 12) & 0xff
		if battery == 1 {
			battery = 100
		} else {
			battery = 10 * (min(battery, 11) - 1)
		}
		batteryVoltage :=  (float64(battery) - 10) * 1.2 / 100 + 1.8

		rawTemp := fromBytesToUint16(md[8:10])
		T := (175.72 * float64(rawTemp) * 4.0 / 65536) - 46.85
		if T < -30 {
			T = -30.0
		} else if T > 70 {
			T = 70.0
		}

		rawPressure := fromBytesToUint16(md[6:8])
		P := float64(rawPressure) / 16.0

		rawHumidity := fromBytesToUint16(md[10:12])
		H := (125 * float64(rawHumidity) * 4.0 / 65536) - 6.0
		if H < 1 {
			H = 1.0
		} else if H > 100 {
			H = 100
		}

		uptime := uint32(fromBytesToUint16(md[12:14])) << 16 | uint32(fromBytesToUint16(md[14:16]))

		fmt.Printf(", B = %d%% (%.1fV), T = %.3fC, P = %.2fhPa, H = %.1f%%, U = %ds",
			battery,
			batteryVoltage,
			T,
			P,
			H,
			uptime)
	}
	fmt.Printf("\n")
}

func chkErr(err error) {
	switch errors.Cause(err) {
	case nil:
	case context.DeadlineExceeded:
		fmt.Printf("done\n")
	case context.Canceled:
		fmt.Printf("canceled\n")
	default:
		log.Fatalf(err.Error())
	}
}