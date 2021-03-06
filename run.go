package main

import (
	"io"
	"io/ioutil"
	"math/rand"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Output struct {
	// [type, msg]
	output [][2]string
	code   int
	info   map[string][]string
}

func runCode(code string) (*Output, error) {
	id := strconv.FormatInt(rand.Int63(), 36)

	err := ioutil.WriteFile("./tmp/"+id+".go", []byte(code), 0777)
	if err != nil {
		return nil, err
	}

	exePath := "./tmp/" + id
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", exePath, "./tmp/"+id+".go")
	_, err = buildCmd.Output()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exePath)
	cmd.Env = []string{"TOKEN=nicetrybud", "GODISC_INSTANCE_ID=" + id}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	messages := make([][2]string, 0)
	mutex := &sync.Mutex{}

	readPipe := func(r io.Reader, t string) {
		b := make([]byte, 1024)
		for {
			n, err := r.Read(b)
			if n != 0 {
				mutex.Lock()
				messages = append(messages, [2]string{t, string(b[:n])})
				mutex.Unlock()
			}
			if err == io.EOF {
				return
			}
		}
	}

	go readPipe(stdoutPipe, "stdout")
	go readPipe(stderrPipe, "stderr")

	err = cmd.Wait()
	ecode := 0
	if eerr, ok := err.(*exec.ExitError); ok {
		ecode = eerr.ExitCode()
	} else if err != nil {
		return nil, err
	}

	info := make(map[string][]string)
	rawInfoBytes, err := ioutil.ReadFile("./tmp/info-" + id + ".txt")
	if err == nil {
		rawInfo := string(rawInfoBytes)
		lines := strings.Split(rawInfo, "\n")
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			i := strings.Index(line, "=")
			if i == -1 {
				logger.Println("warning: invalid line in info file", line)
				continue
			}
			key := line[:i]
			val := line[i+1:]
			info[key] = append(info[key], val)
		}
	}

	return &Output{messages, ecode, info}, nil
}
