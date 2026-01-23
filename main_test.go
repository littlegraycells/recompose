package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMaskValue(t *testing.T) {
	if maskValue("DB_PASSWORD", "secret123") != "********" {
		t.Error("Failed to mask password")
	}
	if maskValue("APP_PORT", "8080") != "8080" {
		t.Error("Incorrectly masked non-sensitive key")
	}
}

func TestFindNode(t *testing.T) {
	yml := "services:\n  web:\n    image: nginx"
	var root yaml.Node
	yaml.Unmarshal([]byte(yml), &root)

	node := findNode(&root, "services")
	if node == nil {
		t.Fatal("Could not find services node")
	}
}
