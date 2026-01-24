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

func TestIntegerPortRefactoring(t *testing.T) {
	yml := "environment:\n  SMTP_PORT: 587"
	var node yaml.Node
	yaml.Unmarshal([]byte(yml), &node)

	// Target the SMTP_PORT value node
	envNode := node.Content[0].Content[1]
	changes := []Change{}
	processEnvBlock("test-service", envNode, &changes)

	valNode := envNode.Content[1]
	if valNode.Tag != "!!str" {
		t.Errorf("Expected Tag to be !!str, got %s", valNode.Tag)
	}
	if valNode.Value != "${SMTP_PORT}" {
		t.Errorf("Expected value ${SMTP_PORT}, got %s", valNode.Value)
	}
}
