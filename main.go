package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	gapi "github.com/grafana/grafana-api-golang-client"
)

type config struct {
	url       string
	apikey    string
	uid       string
	replace   replacements
	overwrite bool
	retries   int
}

var cfg config

func main() {
	flag.StringVar(&cfg.url, "url", "", "Grafana url (required)")
	flag.StringVar(&cfg.apikey, "apikey", "", "Grafana api key (required)")
	flag.StringVar(&cfg.uid, "uid", "", "Dashboard uid to process (required)")
	flag.Var(&cfg.replace, "replace", fmt.Sprintf("What to replace (key%svalue, multiple entries allowed, required)", replacementSeperator))
	flag.BoolVar(&cfg.overwrite, "overwrite", true, "Overwrite dashboard on conflict")
	flag.IntVar(&cfg.retries, "retries", 3, "Retries when grafana the grafana api")

	flag.Parse()

	if err := checkConfig(cfg); err != nil {
		log.Fatal("bad config: ", err)
	}
	if err := processDashboard(cfg); err != nil {
		log.Fatal("unable to process dashboard: ", err)
	}

	log.Println("processed dashboard")
}

func processDashboard(cfg config) error {
	client, err := gapi.New(cfg.url, gapi.Config{
		APIKey:     cfg.apikey,
		NumRetries: cfg.retries,
	})
	if err != nil {
		return fmt.Errorf("unable to create grafana client: %w", err)
	}

	dashboard, err := client.DashboardByUID(cfg.uid)
	if err != nil {
		return fmt.Errorf("unable to fetch dashboards: %w", err)
	}
	dbytes, err := json.Marshal(dashboard.Model)
	if err != nil {
		return fmt.Errorf("unable to marshal model: %w", err)
	}
	for i := range cfg.replace.rs {
		key, val := cfg.replace.rs[i].key, cfg.replace.rs[i].val
		dbytes = bytes.ReplaceAll(dbytes, []byte(key), []byte(val))
	}

	model := make(map[string]interface{})
	if err := json.Unmarshal(dbytes, &model); err != nil {
		return fmt.Errorf("unable to marshal processed model: %w", err)
	}
	dashboard.Model = model
	dashboard.Overwrite = cfg.overwrite
	dashboard.Message = replacerMessage(cfg)

	if _, err := client.NewDashboard(*dashboard); err != nil {
		return fmt.Errorf("unable to save dashboard: %w", err)
	}

	return nil
}

func replacerMessage(cfg config) string {
	const (
		prefix = "String replacement by aiven-grafana-string-replacer"
	)

	b := new(strings.Builder)
	b.WriteString(prefix)
	b.WriteString(": ")
	for i := range cfg.replace.rs {
		if i > 0 {
			b.WriteString(", ")
		}
		key, val := cfg.replace.rs[i].key, cfg.replace.rs[i].val
		b.WriteString(fmt.Sprintf("%s<=>%s", key, val))
	}
	return b.String()
}

type replacement struct {
	key string
	val string
}

func checkConfig(cfg config) error {
	if cfg.url == "" {
		return errors.New("'url' is required")
	}
	if cfg.apikey == "" {
		return errors.New("'apikey' is required")
	}
	if cfg.uid == "" {
		return errors.New("'uid' is required")
	}
	if len(cfg.replace.rs) == 0 {
		return errors.New("'replace' is required")
	}
	return nil
}
