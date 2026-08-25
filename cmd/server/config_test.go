package main

import "testing"

func TestValidateAddressOnlyAllowsLoopbackHighPort(t *testing.T) {
	for _, address := range []string{"127.0.0.1:19081", "localhost:24000", "[::1]:25000"} {
		if err := validateAddress(address); err != nil {
			t.Errorf("应允许 %s：%v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19081", "127.0.0.1:80", "192.168.1.2:19081", "bad"} {
		if err := validateAddress(address); err == nil {
			t.Errorf("不应允许 %s", address)
		}
	}
}

func TestAddressFromPort(t *testing.T) {
	address, err := addressFromPort("19123")
	if err != nil || address != "127.0.0.1:19123" {
		t.Fatalf("PORT 解析异常：%s %v", address, err)
	}
	if _, err := addressFromPort("80"); err == nil {
		t.Fatal("不应允许低位 PORT")
	}
}
