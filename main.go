package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Change struct {
	Service string
	OldLine string
	NewLine string
	Key     string
	Value   string
}

func main() {
	var filePath string
	var genCompose bool
	var genEnv bool
	var overwrite bool

	flag.StringVar(&filePath, "file", "", "Path to the docker-compose file")
	flag.StringVar(&filePath, "F", "", "Shorthand for --file")
	flag.BoolVar(&genCompose, "generate-compose", true, "Generate the refactored compose file")
	flag.BoolVar(&genCompose, "C", true, "Shorthand for --generate-compose")
	flag.BoolVar(&genEnv, "generate-env", true, "Generate the .env file")
	flag.BoolVar(&genEnv, "E", true, "Shorthand for --generate-env")
	flag.BoolVar(&overwrite, "overwrite", true, "Overwrite existing files")
	flag.BoolVar(&overwrite, "O", true, "Shorthand for --overwrite")

	flag.Parse()

	if filePath == "" {
		fmt.Println("Error: --file | -F is required.")
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(filePath)
	baseDir := filepath.Dir(absPath)
	recomposeDir := filepath.Join(baseDir, "recompose")
	newComposePath := filepath.Join(recomposeDir, "compose.yml")
	newEnvPath := filepath.Join(recomposeDir, ".env")

	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	servicesNode := findNode(&root, "services")
	if servicesNode == nil {
		log.Fatal("Validation Failed: No services block found.")
	}

	changes := []Change{}
	for i := 0; i < len(servicesNode.Content); i += 2 {
		serviceName := servicesNode.Content[i].Value
		envNode := findNode(servicesNode.Content[i+1], "environment")
		if envNode != nil {
			processEnvBlock(serviceName, envNode, &changes)
		}
	}

	if len(changes) == 0 {
		fmt.Println("No changes needed.")
		return
	}

	printTable(changes)

	if !genCompose && !genEnv {
		fmt.Println("\nMode: TRY RUN. No files will be generated.")
		return
	}

	// PROMPT: This now uses a standard buffer that waits for the Enter key
	fmt.Printf("\nApply changes? (y/N): ")
	if askConfirmation() {
		os.MkdirAll(recomposeDir, 0755)
		if genEnv {
			handleWrite(newEnvPath, overwrite, func() { writeFormattedEnvFile(newEnvPath, changes) })
		}
		if genCompose {
			handleWrite(newComposePath, overwrite, func() {
				out, _ := yaml.Marshal(&root)
				ioutil.WriteFile(newComposePath, out, 0644)
			})
		}
	} else {
		fmt.Println("Aborted. No files written.")
	}
}

// askConfirmation: Standard buffered input that waits for 'Enter'
func askConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		return true
	}
	return false
}

// --- REMAINING HELPERS (findNode, processEnvBlock, printTable, handleWrite, writeFormattedEnvFile) ---

func handleWrite(path string, canOverwrite bool, writeFunc func()) {
	if _, err := os.Stat(path); err == nil && !canOverwrite {
		fmt.Printf("Skipped: %s (File exists and overwrite is false)\n", filepath.Base(path))
		return
	}
	writeFunc()
	fmt.Printf("- Created: %s\n", path)
}

func findNode(node *yaml.Node, key string) *yaml.Node {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 { return nil }
		return findNode(node.Content[0], key)
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key { return node.Content[i+1] }
		}
	}
	return nil
}

func processEnvBlock(service string, envNode *yaml.Node, changes *[]Change) {
	if envNode.Kind == yaml.MappingNode {
		for i := 0; i < len(envNode.Content); i += 2 {
			k, v := envNode.Content[i], envNode.Content[i+1]
			if !strings.Contains(v.Value, "${") {
				*changes = append(*changes, Change{
					Service: service, OldLine: k.Value + ": " + v.Value,
					NewLine: k.Value + ": ${" + k.Value + "}", Key: k.Value, Value: v.Value,
				})
				v.Value = "${" + k.Value + "}"
			}
		}
	} else if envNode.Kind == yaml.SequenceNode {
		for _, item := range envNode.Content {
			parts := strings.SplitN(item.Value, "=", 2)
			if len(parts) == 2 && !strings.Contains(parts[1], "${") {
				*changes = append(*changes, Change{
					Service: service, OldLine: item.Value,
					NewLine: parts[0] + "=${" + parts[0] + "}", Key: parts[0], Value: parts[1],
				})
				item.Value = parts[0] + "=${" + parts[0] + "}"
			}
		}
	}
}

func printTable(changes []Change) {
	wSrv, wOld, wNew := 22, 40, 40
	line := fmt.Sprintf("├%s┼%s┼%s┤", strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2))
	fmt.Printf("\n┌%s┬%s┬%s┐\n", strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2))
	fmt.Printf("│ %-22s │ %-40s │ %-40s │\n", "SERVICE", "CURRENT LINE", "PROPOSED LINE")
	fmt.Printf("╞%s╪%s╪%s╡\n", strings.Repeat("=", wSrv+2), strings.Repeat("=", wOld+2), strings.Repeat("=", wNew+2))
	for i, c := range changes {
		fmt.Printf("│ %-22.22s │ %-40.40s │ %-40.40s │\n", c.Service, c.OldLine, c.NewLine)
		if i < len(changes)-1 && changes[i+1].Service != c.Service { fmt.Println(line) }
	}
	fmt.Printf("└%s┴%s┴%s┘\n", strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2))
}

func writeFormattedEnvFile(path string, changes []Change) {
	keyUsage := make(map[string]map[string]int)
	serviceVars := make(map[string][]string)
	for _, c := range changes {
		if _, ok := keyUsage[c.Key]; !ok { keyUsage[c.Key] = make(map[string]int) }
		keyUsage[c.Key][c.Value]++
	}
	globals := []string{}
	processedGlobals := make(map[string]bool)
	for key, valMap := range keyUsage {
		for val, count := range valMap {
			if count > 1 {
				globals = append(globals, key+"="+val)
				processedGlobals[key] = true
			}
		}
	}
	sort.Strings(globals)
	for _, c := range changes {
		if !processedGlobals[c.Key] {
			serviceVars[c.Service] = append(serviceVars[c.Service], c.Key+"="+c.Value)
		}
	}
	var sb strings.Builder
	sb.WriteString("# GLOBAL VARIABLES\n")
	for _, g := range globals { sb.WriteString(g + "\n") }
	var services []string
	for s := range serviceVars { services = append(services, s) }
	sort.Strings(services)
	for _, s := range services {
		sb.WriteString(fmt.Sprintf("\n# SERVICE: %s\n", strings.ToUpper(s)))
		sort.Strings(serviceVars[s])
		for _, v := range serviceVars[s] { sb.WriteString(v + "\n") }
	}
	ioutil.WriteFile(path, []byte(sb.String()), 0644)
}
