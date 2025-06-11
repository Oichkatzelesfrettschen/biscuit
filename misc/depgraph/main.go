package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
)

// Program depgraph generates a Graphviz DOT description of the module dependency graph.
//
// @return None. The DOT graph is printed to standard output. Any error from
// `go mod graph` results in panic.
func main() {
	cmd := exec.Command("go", "mod", "graph")
	output, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	writer.WriteString("digraph deps {\n")
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			continue
		}
		writer.WriteString("    \"" + string(fields[0]) + "\" -> \"" + string(fields[1]) + "\";\n")
	}
	writer.WriteString("}\n")
}
