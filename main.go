package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

var pid string
var length int64
var file string
var interval int

func main() {
	flag.StringVar(&pid, "pid", "", "pid to monitor")
	flag.Int64Var(&length, "time", 0, "how many seconds the monitor should run for")
	flag.StringVar(&file, "file", "data", "file to write data to")
	flag.IntVar(&interval, "interval", 2, "interval in seconds")
	flag.Parse()

	if pid == "" {
		fmt.Println("please specify a pid")
		os.Exit(0)
	}

	results := make(map[string]map[string][]string)

	results[pid] = make(map[string][]string)
	results[pid]["cpu"] = make([]string, 0)
	results[pid]["mem"] = make([]string, 0)

	out, err := exec.Command("pgrep", "-P", pid).Output()
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
						os.Exit(1)
					}

					s := strings.Split(string(out), "\n")

					v := strings.Split(strings.Replace(strings.Trim(s[1], " "), "  ", " ", -1), " ")
					mem := v[0]
					cpu := v[1]
					results[pid]["mem"] = append(results[pid]["mem"], mem)
					results[pid]["cpu"] = append(results[pid]["cpu"], cpu)
					wg.Done()
				}(k)

			}
			wg.Wait()
		case <-stopper.C:
			writeResults(results, file)
			os.Exit(0)
		}
	}

	writeResults(results, file)

}

func writeResults(results map[string]map[string][]string, file string) {
	f, err := os.Create(file)

	if err != nil {
		fmt.Printf("could not open file %s for writing\n", file)
		os.Exit(1)
	}
	keys := []string{}
	for k, _ := range results {
		keys = append(keys, k)
	}
	row := make([]string, 0)
	for _, k := range keys {
		row = append(row, k)
	}

	for i := 0; i < len(results[pid]["mem"]); i++ {

		row = []string{strconv.Itoa(i + 1)}

		for _, k := range keys {
			row = append(row, results[k]["mem"][i])
		}
		f.WriteString(strings.Join(row, ",    "))
		f.WriteString("\n")
	}
	writeGnuPlotFile(keys, file)
}

type Row struct {
	Index int
}

func writeGnuPlotFile(keys []string, filename string) {
	var gnuTemplate = fmt.Sprintf(`set term png
set output "graph.png"
plot {{range .}}'%s' using 1:{{.Index}} with lines, {{end}}`, filename)

	t := template.New("t")
	parsed, err := t.Parse(gnuTemplate)
	if err != nil {
		fmt.Println("something went wrong with templating")
	}

	f, err := os.Create("template.gp")
	if err != nil {
		fmt.Println("could not open file for writing the template")
	}

	rows := make([]Row, 0)

	for i, _ := range keys {
		rows = append(rows, Row{Index: i + 2})
	}
	parsed.Execute(f, rows)
}
