package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v2"
)

// Flags .
type Flags struct {
	URL string

	TestsFile string
	Runs      int
	OutDir    string
}

func (f *Flags) Register(fs *flag.FlagSet) {
	fs.StringVar(&f.URL, "test.url", "", "ws/wss url to test")
	fs.StringVar(&f.OutDir, "dir", "", "directory to output result files into")
	fs.StringVar(&f.TestsFile, "tests", "", "tests file to load urls/rules from")
	fs.IntVar(&f.Runs, "n", 1, "number of time to run each test (0=run forever)")

}

func loadTestsFile(filename string) (TestsFile, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return TestsFile{}, err
	}

	var tf TestsFile

	if err := yaml.Unmarshal(data, &tf); err != nil {
		return TestsFile{}, err
	}
	return tf, nil

}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Minute

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var flags Flags
	flags.Register(flag.CommandLine)
	flag.Parse()
	if flags.URL != "" && flags.TestsFile != "" {
		fmt.Println("-url and -tests are mutually exclusive")
		os.Exit(1)
	}

	var tests TestsFile
	if flags.URL != "" {
		tests = append(tests, Test{
			Name:               "commandline",
			URL:                flags.URL,
			HandshakeTimeout:   Duration(20 * time.Minute),
			Sleep:              Duration(5 * time.Minute),
			MessageReadTimeout: Duration(10 * time.Minute),
		})
	}
	if flags.TestsFile != "" {
		tf, err := loadTestsFile(flags.TestsFile)
		if err != nil {
			fmt.Println("error loading tests file", err)
			os.Exit(1)
		}
		tests = append(tests, tf...)
	}

	if len(tests) < 1 {
		fmt.Println("no tests found, use -url or -tests to load")
		os.Exit(1)
	}

	if flags.OutDir != "" {
		if err := os.MkdirAll(flags.OutDir, 0700); err != nil {
			log.Fatalln("could not create outdir directory", err)
		}

	}

	var wg sync.WaitGroup

	for _, v := range tests {
		wt := v
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
		loop:
			for {
				if flags.Runs > 0 {
					if n >= flags.Runs {
						break loop
					}
				}
				wr, err := testWS(context.Background(), wt)
				if err != nil {
					log.Println("failed ws test", err)
				} else {
					if !wr.IsSuccess() {
						log.Println("TEST UNSUCCESSFUL")
					}
					// spew.Dump(wr)
					// data, err := yaml.Marshal(&wr)
					data, err := json.MarshalIndent(&wr, "", "  ")
					if err != nil {
						log.Fatal(err)
					}
					if flags.OutDir != "" {
						filename := filepath.Join(
							flags.OutDir,
							fmt.Sprintf("%v__%v.json",
								wr.Test.Name,
								wr.StartedAt.Format("2006-01-02__150405.999999999"),
							),
						)
						if err := ioutil.WriteFile(filename, data, 0600); err != nil {
							log.Fatal(err)
						}
					}
					log.Println(wr.IsSuccess(), string(data))
				}
				time.Sleep(time.Duration(wt.Sleep))
				n++
			}
		}()
	}
	wg.Wait()
}
