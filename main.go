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

var Version = "v0.0.1"

const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorRed    = "\033[31m"
	ColorGray   = "\033[90m"
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
	var genCompose, genEnv, showVersion bool

	flag.StringVar(&filePath, "file", "", "Path to the docker-compose file")
	flag.StringVar(&filePath, "F", "", "Shorthand for --file")
	flag.BoolVar(&genCompose, "generate-compose", true, "Generate the refactored compose file")
	flag.BoolVar(&genEnv, "generate-env", true, "Generate the .env file")
	flag.BoolVar(&showVersion, "version", false, "Print version information")
	flag.BoolVar(&showVersion, "V", false, "Shorthand for --version")
	flag.Parse()

	if showVersion {
		fmt.Printf("Recompose %s\n", Version)
		return
	}

	if filePath == "" {
		fmt.Printf("%sError: --file | -F is required.%s\n", ColorRed, ColorReset)
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

	applyTopLevelSpacing(&root)

	servicesNode := findNode(&root, "services")
	if servicesNode == nil {
		log.Fatal("Validation Failed: No services block found.")
	}

	changes := []Change{}
	for i := 0; i < len(servicesNode.Content); i += 2 {
		serviceName := servicesNode.Content[i].Value
		if i > 0 {
			servicesNode.Content[i].HeadComment = "\n"
		}
		envNode := findNode(servicesNode.Content[i+1], "environment")
		if envNode != nil {
			processEnvBlock(serviceName, envNode, &changes)
		}
	}

	if len(changes) == 0 {
		fmt.Println("No hardcoded environment variables found. No changes needed.")
		return
	}

	printTable(changes)

	if !genCompose && !genEnv {
		fmt.Printf("\n%sMode: TRY RUN. No files will be generated.%s\n", ColorCyan, ColorReset)
		return
	}

	fmt.Printf("\nApply changes? (y/N): ")
	if askConfirmation() {
		os.MkdirAll(recomposeDir, 0755)
		if genEnv {
			writeFormattedEnvFile(newEnvPath, changes)
			fmt.Printf("%s- Created/Replaced: %s%s\n", ColorGreen, newEnvPath, ColorReset)
		}
		if genCompose {
			f, _ := os.Create(newComposePath)
			trimmedData := strings.TrimSpace(string(data))
			if !strings.HasPrefix(trimmedData, "---") {
				f.WriteString("---\n")
			}
			enc := yaml.NewEncoder(f)
			enc.SetIndent(2)
			enc.Encode(&root)
			f.Close()
			fmt.Printf("%s- Created/Replaced: %s%s\n", ColorGreen, newComposePath, ColorReset)
		}
	}
}

func applyTopLevelSpacing(root *yaml.Node) {
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		topMap := root.Content[0]
		for i := 2; i < len(topMap.Content); i += 2 {
			topMap.Content[i].HeadComment = "\n"
		}
	}
}

func maskValue(key, value string) string {
	lowKey := strings.ToLower(key)
	sensitives := []string{"pass", "secret", "token", "key", "auth"}
	for _, s := range sensitives {
		if strings.Contains(lowKey, s) {
			return "********"
		}
	}
	return value
}

func printTable(changes []Change) {
	wSrv, wOld, wNew := 18, 35, 35
	fmt.Printf("\n%s┌%s┬%s┬%s┐%s\n", ColorGray, strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2), ColorReset)
	fmt.Printf("│ %-18s │ %-35s │ %-35s │\n", "SERVICE", "CURRENT (MASKED)", "PROPOSED")
	fmt.Printf("%s╞%s╪%s╪%s╡%s\n", ColorGray, strings.Repeat("=", wSrv+2), strings.Repeat("=", wOld+2), strings.Repeat("=", wNew+2), ColorReset)

	for i, c := range changes {
		maskedVal := maskValue(c.Key, c.Value)
		displayOld := fmt.Sprintf("%s: %s", c.Key, maskedVal)
		fmt.Printf("│ %-18.18s │ %s%-35.35s%s │ %s%-35.35s%s │\n",
			c.Service, ColorYellow, displayOld, ColorReset, ColorGreen, c.NewLine, ColorReset)
		if i < len(changes)-1 && changes[i+1].Service != c.Service {
			fmt.Printf("%s├%s┼%s┼%s┤%s\n", ColorGray, strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2), ColorReset)
		}
	}
	fmt.Printf("%s└%s┴%s┴%s┘%s\n", ColorGray, strings.Repeat("-", wSrv+2), strings.Repeat("-", wOld+2), strings.Repeat("-", wNew+2), ColorReset)
}

func writeFormattedEnvFile(path string, changes []Change) {
	keyUsage := make(map[string]map[string]int)
	serviceVars := make(map[string][]string)
	for _, c := range changes {
		if _, ok := keyUsage[c.Key]; !ok {
			keyUsage[c.Key] = make(map[string]int)
		}
		keyUsage[c.Key][c.Value]++
	}

	var sb strings.Builder
	sb.WriteString("# RECOMPOSE GENERATED ENV\n")
	sb.WriteString("# GLOBAL VARIABLES\n")

	globals := []string{}
	processedGlobals := make(map[string]bool)
	for key, valMap := range keyUsage {
		if len(valMap) > 1 || (len(valMap) == 1 && getFirstValCount(valMap) > 1) {
			for val := range valMap {
				globals = append(globals, fmt.Sprintf("%s=%s", key, val))
			}
			processedGlobals[key] = true
		}
	}
	sort.Strings(globals)
	for _, g := range globals {
		sb.WriteString(g + "\n")
	}

	for _, c := range changes {
		if !processedGlobals[c.Key] {
			serviceVars[c.Service] = append(serviceVars[c.Service], fmt.Sprintf("%s=%s", c.Key, c.Value))
		}
	}

	srvKeys := make([]string, 0, len(serviceVars))
	for k := range serviceVars {
		srvKeys = append(srvKeys, k)
	}
	sort.Strings(srvKeys)

	for _, srv := range srvKeys {
		sb.WriteString(fmt.Sprintf("\n# SERVICE: %s\n", strings.ToUpper(srv)))
		sort.Strings(serviceVars[srv])
		for _, v := range serviceVars[srv] {
			sb.WriteString(v + "\n")
		}
	}
	ioutil.WriteFile(path, []byte(sb.String()), 0644)
}

func getFirstValCount(m map[string]int) int {
	for _, v := range m {
		return v
	}
	return 0
}

func findNode(node *yaml.Node, key string) *yaml.Node {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return findNode(node.Content[0], key)
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
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
					Service: service,
					Key:     k.Value,
					Value:   v.Value,
					NewLine: k.Value + ": ${" + k.Value + "}",
				})

				v.Value = "${" + k.Value + "}"
				v.Tag = "!!str"
			}
		}
	} else if envNode.Kind == yaml.SequenceNode {
		for _, item := range envNode.Content {
			parts := strings.SplitN(item.Value, "=", 2)
			if len(parts) == 2 && !strings.Contains(parts[1], "${") {
				*changes = append(*changes, Change{
					Service: service,
					Key:     parts[0],
					Value:   parts[1],
					NewLine: parts[0] + "=${" + parts[0] + "}",
				})

				item.Value = parts[0] + "=${" + parts[0] + "}"
				item.Tag = "!!str"
			}
		}
	}
}

func askConfirmation() bool {
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(response)) == "y"
}
