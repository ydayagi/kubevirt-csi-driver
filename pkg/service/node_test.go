package service

import "testing"

func TestDeviceExtraction(t *testing.T) {

	device, err := getDeviceBySerialID("S35ENX0J663758")
	t.Log(err)
	t.Logf("device %+v", device)

}
