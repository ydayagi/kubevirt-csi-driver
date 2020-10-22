package service

import "testing"

func TestDeviceExtraction(t *testing.T) {

	device, err := getDeviceBySerialID("werwerwr")
	t.Log(err)
	t.Log(device)

}
