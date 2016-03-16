package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var processName string
var length int64
var file string
var interval int
var now = time.Now()
var layout = "20060102150405"
var cpuFileName = fmt.Sprintf("cpu_%s", now.Format(layout))
var memFileName = fmt.Sprintf("mem_%s", now.Format(layout))
var whiteSpace = regexp.MustCompile(` +`)

func main() {
	flag.StringVar(&processName, "name", "", "processName to monitor")
	flag.Int64Var(&length, "time", 0, "how many seconds the monitor should run for")
	flag.StringVar(&file, "file", "data", "file to write data to")
	flag.IntVar(&interval, "interval", 2, "interval in seconds")
	flag.Parse()

	if processName == "" {
		fmt.Println("please specify a process name")
		os.Exit(0)
	}

	results := make(map[string]map[string][]string)

	out, err := exec.Command("pgrep", processName).Output()
	if strings.Trim(string(out), " ") != "" {
		childProcessesList := strings.Split(string(out), "\n")

		for _, v := range childProcessesList {
			if strings.Trim(v, " ") == "" {
				continue
			}
			results[v] = make(map[string][]string)
			results[v]["cpu"] = make([]string, 0)
			results[v]["mem"] = make([]string, 0)
		}
	}

	t := time.NewTicker(time.Duration(interval) * time.Second)

	stopper := time.NewTimer(time.Duration(length) * time.Second)
	length := 0
	for {
		select {
		case <-t.C:
			wg := sync.WaitGroup{}
			for k := range results {
				wg.Add(1)
				go func(pid string) {
					out, err = exec.Command("ps", "-p", pid, "-o", "%mem,%cpu").Output()
					if err != nil {
						fmt.Printf("oh no error: %s output: %s\n", err.Error(), string(out))
						writeResults(results, length)
						os.Exit(1)
					}

					s := strings.Split(string(out), "\n")
					v := whiteSpace.Split(strings.Trim(s[1], " "), -1)
					mem := v[0]
					cpu := v[1]
					results[pid]["mem"] = append(results[pid]["mem"], mem)
					results[pid]["cpu"] = append(results[pid]["cpu"], cpu)
					wg.Done()
				}(k)

			}
			wg.Wait()
			length++
		case <-stopper.C:
			writeResults(results, length)
			os.Exit(0)
		}
	}

	writeResults(results, length)
}

func writeResults(results map[string]map[string][]string, length int) {
	cpuFile, err := os.Create(cpuFileName)
	if err != nil {
		fmt.Printf("could not open file %s for writing\n", cpuFile)
		os.Exit(1)
	}
	memFile, err := os.Create(memFileName)

	if err != nil {
		fmt.Printf("could not open file %s for writing\n", memFile)
		os.Exit(1)
	}

	keys := []string{}
	for k, _ := range results {
		keys = append(keys, k)
	}
	memRow := make([]string, 0)
	cpuRow := make([]string, 0)
	for _, k := range keys {
		memRow = append(memRow, k)
		cpuRow = append(cpuRow, k)
	}

	for i := 0; i < length; i++ {

		memRow = []string{strconv.Itoa(i + 1)}
		cpuRow = []string{strconv.Itoa(i + 1)}

		for _, k := range keys {
			memRow = append(memRow, results[k]["mem"][i])
			cpuRow = append(cpuRow, results[k]["cpu"][i])
		}
		cpuFile.WriteString(strings.Join(cpuRow, ",    "))
		cpuFile.WriteString("\n")
		memFile.WriteString(strings.Join(memRow, ",    "))
		memFile.WriteString("\n")

	}
	writeGnuPlotFile(keys)
}

type Row struct {
	Index int
}

func writeGnuPlotFile(keys []string) {
	var gnuTemplate = fmt.Sprintf(`set term png
set output "graph_mem.png"
plot {{range .}}'%s' using 1:{{.Index}} with lines, {{end}}
set term png
set output "graph_cpu.png"
plot {{range .}}'%s' using 1:{{.Index}} with lines, {{end}}`, memFileName, cpuFileName)

	t := template.New("t")
	parsed, err := t.Parse(gnuTemplate)
	if err != nil {
		fmt.Println("something went wrong with templating")
	}

	f, err := os.Create(fmt.Sprintf("template_%s.gp", now.Format(layout)))
	if err != nil {
		fmt.Println("could not open file for writing the template")
	}

	rows := make([]Row, 0)

	for i, _ := range keys {
		rows = append(rows, Row{Index: i + 2})
	}
	parsed.Execute(f, rows)
}
