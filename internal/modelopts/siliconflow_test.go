package modelopts

import "testing"

func TestProviderIsolation(t *testing.T) {
	if Extra("https://api.siliconflow.cn/v1", "deepseek-ai/DeepSeek-V4-Pro")["enable_thinking"] != true {
		t.Fatal("missing compatibility option")
	}
	for _, base := range []string{"https://other.example/v1", "https://api.siliconflow.cn.attacker.test/v1", "://"} {
		if Extra(base, "deepseek-ai/DeepSeek-V4-Pro") != nil {
			t.Fatal("option leaked to another provider")
		}
	}
}
