package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	// "regexp"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// connBytes holds rx and tx byte counters as strings.
type connBytes struct {
	rx string
	tx string
}

// Return a map keyed by a string constructed as
// "localIP:localPort->remoteIP:remotePort".
func parseSSOutput() (map[string]connBytes, error) {
	data := make(map[string]connBytes)

	out, err := exec.Command("ss", "-tuln", "-o", "state", "established").Output()
	if err != nil {
		fmt.Printf("Error running ss command: %v\n", err)
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	var currentKey string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if line[0] != ' ' && line[0] != '\t' {
			//	Netid	Recv-Q	Send-Q	Local Address:Port	Peer Address:Port	Process
			//	tcp		0		0		127.0.0.1:49911		127.0.0.1:42488     users:(("python",pid=802085,fd=31))
			fields := strings.Fields(line)
			if len(fields) < 5 {
				fmt.Printf("Didn't find enough fields in line: %s\n", line)
				currentKey = ""
				continue
			}
			recvq := fields[1]
			sendq := fields[2]
			local := fields[3]
			remote := fields[4]
			currentKey = local + "->" + remote
			// fmt.Printf("Recv-Q: %s, Send-Q: %s, Local Address: %s, Peer Address: %s\n", recvq, sendq, local, remote)

			if currentKey != "" {
				data[currentKey] = connBytes{rx: recvq, tx: sendq}
			}
		}
	}
	return data, scanner.Err()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: netmon <app-name>")
		os.Exit(1)
	}
	appName := os.Args[1]
	interval := 3

	for {
		var ssData map[string]connBytes
		if runtime.GOOS == "linux" {
			var err error
			ssData, err = parseSSOutput()
			if err != nil {
				log.Printf("Warning: could not parse ss output: %v", err)
			}
		}

		procs, err := process.Processes()
		if err != nil {
			log.Fatalf("Error retrieving processes: %v", err)
		}

		fmt.Printf("%-6s %-15s %-22s %-22s %-12s %-10s %-10s\n", "PID", "Process", "Local Address", "Remote Address", "State", "Rx Bytes", "Tx Bytes")
		for _, p := range procs {
			name, err := p.Name()
			if err != nil {
				continue
			}
			// Use case-insensitive substring matching.
			if !strings.Contains(strings.ToLower(name), strings.ToLower(appName)) {
				// fmt.Printf("name (%s) does not contain appName (%s)\n", name, appName)
				continue
			}

			pid := p.Pid
			conns, err := p.Connections()
			// fmt.Printf("conns: %v\n", conns)
			if err != nil {
				continue
			}
			for _, conn := range conns {
				local := fmt.Sprintf("%s:%d", conn.Laddr.IP, conn.Laddr.Port)
				remote := fmt.Sprintf("%s:%d", conn.Raddr.IP, conn.Raddr.Port)
				state := conn.Status

				rxVal, txVal := "N/A", "N/A"
				if runtime.GOOS == "linux" && ssData != nil {
					// fmt.Printf("In main loop, when GOOS==Linux, local: %s, remote: %s\n", local, remote)
					key := local + "->" + remote
					if vals, ok := ssData[key]; ok {
						rxVal, txVal = vals.rx, vals.tx
					}
				}
				fmt.Printf("%-6d %-15s %-22s %-22s %-12s %-10s %-10s\n", pid, name, local, remote, state, rxVal, txVal)
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
		clearScreen()
	}
}

func clearScreen() {
	var cmd *exec.Cmd

	if (runtime.GOOS == "linux") || (runtime.GOOS == "darwin") {
		cmd = exec.Command("clear")
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		fmt.Println("Unsupported operating system for clearing the screen.")
		return
	}

	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error clearing the screen:", err)
	}
}
