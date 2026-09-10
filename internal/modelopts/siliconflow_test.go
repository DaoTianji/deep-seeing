package modelopts

import (
	"reflect"
	"testing"
)

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

func TestOfficialDeepSeekOptions(t *testing.T) {
	want := map[string]any{"thinking": map[string]any{"type": "enabled"}, "reasoning_effort": "high"}
	for _, model := range []string{"deepseek-v4-pro", "deepseek-flash", "deepseek-v4-flash"} {
		for _, base := range []string{"https://api.deepseek.com", "https://api.deepseek.com/v1"} {
			if got := Extra(base, model); !reflect.DeepEqual(got, want) {
				t.Fatalf("official option mismatch: %s %s", base, model)
			}
		}
	}
	for _, base := range []string{"https://api.deepseek.com.attacker.test/v1", "https://other.example/v1", "http://api.deepseek.com/v1", "https://user@api.deepseek.com/v1"} {
		if Extra(base, "deepseek-v4-pro") != nil {
			t.Fatal("official options leaked to untrusted endpoint")
		}
	}
	if Extra("https://api.deepseek.com/v1", "deepseek-ai/DeepSeek-V4-Pro") != nil {
		t.Fatal("SiliconFlow model ID must not be treated as an official model ID")
	}
	if Extra("https://api.siliconflow.cn/v1", "deepseek-v4-pro") != nil {
		t.Fatal("official model ID must not be treated as a SiliconFlow model ID")
	}
}
