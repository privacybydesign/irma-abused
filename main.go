// (c) 2018 - Bas Westerbaan <bas@westerbaan.name>
// You may redistribute this file under the conditions of the GPLv3.

// irma-abused is a simple webserver that collects abuse notifications
// from users of the IRMA app.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/privacybydesign/irmago"

	"github.com/jinzhu/gorm"
	"gopkg.in/yaml.v2"

	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// Globals
var (
	conf Conf
	db   *gorm.DB
)

// Configuration
type Conf struct {
	DB       string // type of database, eg. "mysql"
	DSN      string // DSN, eg.  "dbuser:password@/database"
	BindAddr string // address to bind to, eg. ":8383"
}

// Data sent along with a "/submit" POST request.
type AbuseReport struct {
	Type          string // "disclosure", "signing"
	Requestor     string
	APIServer     string
	AttrDisjList  irma.AttributeDisjunctionList
	ReporterEmail *string
}

// Data of an abuse report stored in the database
type AbuseRecord struct {
	When          time.Time
	Type          string // "disclosure", "signing"
	Requestor     string
	APIServer     string
	AttrDisjList  []byte `sql:"type:text;"`
	ReporterEmail *string
}

func main() {
	var confPath string

	// config defaults
	conf.BindAddr = "localhost:8383"

	// parse commandline
	flag.StringVar(&confPath, "config", "config.yaml",
		"Path to configuration file")
	flag.Parse()

	// parse configuration file
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		fmt.Printf("Could not find config file: %s\n", confPath)
		fmt.Println("It should look like")
		fmt.Println("")
		fmt.Println("   db: mysql")
		fmt.Println("   dsn: dbuser:password@/database")
		fmt.Println("   bindaddr: ':8383'")
		os.Exit(1)
		return
	}

	buf, err := ioutil.ReadFile(confPath)
	if err != nil {
		log.Fatalf("Could not read config file %s: %s", confPath, err)
	}

	if err := yaml.Unmarshal(buf, &conf); err != nil {
		log.Fatalf("Could not parse config file: %s", err)
	}

	// connect to database
	log.Println("Connecting to database ...")
	db, err = gorm.Open(conf.DB, conf.DSN)

	if err != nil {
		log.Fatalf(" %s: could not connect to %s: %s", conf.DB, conf.DSN, err)
	}
	defer db.Close()
	log.Println(" ok")

	log.Println("Auto-migration (if necessary) ...")
	db.AutoMigrate(AbuseRecord{})
	log.Println(" ok")

	// set up HTTP server
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi, this is irma-abused.")
	})

	log.Printf("Listening on %s", conf.BindAddr)

	log.Fatal(http.ListenAndServe(conf.BindAddr, nil))
}

// Handle /submit HTTP requests used to submit events
func submitHandler(w http.ResponseWriter, r *http.Request) {
	var report AbuseReport

	err := json.Unmarshal([]byte(r.FormValue("report")), &report)
	if err != nil {
		http.Error(w, fmt.Sprintf(
			"Missing or malformed report form field: %s", err), 400)
		return
	}

	attrDisjList, _ := json.Marshal(&report.AttrDisjList)

	record := AbuseRecord{
		When:          time.Now(),
		Type:          report.Type,
		Requestor:     report.Requestor,
		APIServer:     report.APIServer,
		AttrDisjList:  attrDisjList,
		ReporterEmail: report.ReporterEmail,
	}
	if err := db.Create(&record).Error; err != nil {
		log.Printf("Failed to store report: %s", err)
	}
}
