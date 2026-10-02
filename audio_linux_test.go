//go:build linux

package main

import "testing"

func TestALSADevicePattern(t *testing.T) {
	output := `**** List of CAPTURE Hardware Devices ****
card 2: Device [USB PnP Sound Device], device 0: USB Audio [USB Audio]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
`
	matches := alsaDevicePattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 1 {
		t.Fatalf("mendapat %d perangkat, ingin 1", len(matches))
	}
	if matches[0][1] != "2" || matches[0][3] != "0" {
		t.Fatalf("card/device salah: %#v", matches[0])
	}
}

func TestLinuxProcessProbe(t *testing.T) {
	probe, err := newProcessProbe()
	if err != nil {
		t.Fatal(err)
	}
	_, ram, err := probe.sample()
	if err != nil {
		t.Fatal(err)
	}
	if ram == 0 {
		t.Fatal("RAM proses seharusnya lebih besar dari nol")
	}
}
