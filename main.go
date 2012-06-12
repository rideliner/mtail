// Copyright 2011 Google Inc. All Rights Reserved.
// This file is available under the Apache license.

package main

import (
	"encoding/csv"
	//"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "net/http/pprof"
)

var port *string = flag.String("port", "3903", "HTTP port to listen on.")
var logs *string = flag.String("logs", "", "List of files to monitor.")
var progs *string = flag.String("progs", "", "Directory containing programs")

var line_count = expvar.NewInt("line_count")

// CSV export
func handleCsv(w http.ResponseWriter, r *http.Request) {
	metric_lock.RLock()
	defer metric_lock.Unlock()
	c := csv.NewWriter(w)
	// for _, v := range vms {
	// 	// for _, m := range v.metrics {
	// 	// 	// record := []string{m.Name,
	// 	// 	// 	fmt.Sprintf("%d", m.Kind),
	// 	// 	// 	m.Unit}
	// 	// 	// switch v := m.(type) {
	// 	// 	// case ScalarMetric:
	// 	// 	// 	d := v.Values["  "]
	// 	// 	// 	record = append(record, fmt.Sprintf("%s", d.Value))
	// 	// 	// 	record = append(record, fmt.Sprintf("%s", d.Time))
	// 	// 	// case DimensionedMetric:
	// 	// 	// 	for k, d := range m.Values {
	// 	// 	// 		keyvals := key_unhash(k)
	// 	// 	// 		for i, key := range m.Keys {
	// 	// 	// 			record = append(record, fmt.Sprintf("%s=%s", key, keyvals[i]))
	// 	// 	// 		}
	// 	// 	// 		record = append(record, fmt.Sprintf("%s", d.Value))
	// 	// 	// 		record = append(record, fmt.Sprintf("%s", d.Time))

	// 	// 	// 	}
	// 	// 	// }
	// 	// 	//c.Write(record)
	// 	// }
	// }
	c.Flush()
}

// JSON export
func handleJson(w http.ResponseWriter, r *http.Request) {
	metric_lock.RLock()
	defer metric_lock.Unlock()
	// for _, v := range vms {
	// 	// b, err := json.Marshal(v.metrics)
	// 	// if err != nil {
	// 	// 	log.Println("error marshalling %s metrics into json:", v.name, err.Error())
	// 	// }
	// 	// w.Write(b)
	// }
}

// RunVms receives a line from a channel and sends it to all VMs.
func RunVms(vms []*vm, lines chan string) {
	for {
		select {
		case line := <-lines:
			line_count.Add(1)
			for _, v := range vms {
				go v.Run(line)
			}
		}
	}
}

// vms contains a list of virtual machines to execute when each new line is received
var (
	vms []*vm
)

func main() {
	flag.Parse()
	w := NewWatcher()
	t := NewTailer(w)

	fis, err := ioutil.ReadDir(*progs)
	if err != nil {
		log.Fatalf("Failure reading progs from %q: %s", *progs, err)
	}

	errors := 0
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		if filepath.Ext(fi.Name()) != "em" {
			continue
		}
		f, err := os.Open(fmt.Sprintf("%s/%s", *progs, fi.Name()))
		if err != nil {
			log.Printf("Failed to open %s: %s\n", fi.Name(), err)
			continue
		}
		defer f.Close()
		vm, errs := Compile(fi.Name(), f)
		if errs != nil {
			errors = 1
			for _, e := range errs {
				log.Print(e)
			}
			continue
		}
		vms = append(vms, vm)
	}

	if *compile_only {
		os.Exit(errors)
	}

	go RunVms(vms, t.Line)
	go t.start()
	go w.start()

	for _, pathname := range strings.Split(*logs, ",") {
		t.Tail(pathname)
	}

	http.HandleFunc("/json", handleJson)
	http.HandleFunc("/csv", handleCsv)
	log.Fatal(http.ListenAndServe(":"+*port, nil))

}
