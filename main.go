package main

import (
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// ----------------------------------------------------------------
// Actuator stuff's
// ----------------------------------------------------------------

// StartupReport represents the spring actuator startup report.
type StartupReport struct {
	SpringBootVersion string   `json:"springBootVersion"`
	Timeline          Timeline `json:"timeline"`
}

// Tags represents the springboot startup step tags.
type Tags struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StartupStep represents the springboot startup step.
type StartupStep struct {
	Name     string `json:"name"`
	ID       int    `json:"id"`
	ParentID int    `json:"parentId"`
	Tags     []Tags `json:"tags"`
}

// Events represents the springboot startup timeline events.
type Events struct {
	StartupStep StartupStep `json:"startupStep"`
	StartTime   time.Time   `json:"startTime"`
	EndTime     time.Time   `json:"endTime"`
}

// Duration calculates the startup time of the event.
func (e Events) Duration() time.Duration {
	return e.EndTime.Sub(e.StartTime)
}

// Timeline represents the springboot startup timeline.
type Timeline struct {
	StartTime time.Time `json:"startTime"`
	Events    []Events  `json:"events"`
}

// Duration calculates the timeline duration.
func (t Timeline) Duration() time.Duration {
	// get max endTime.
	var max time.Time
	for _, e := range t.Events {
		if e.EndTime.After(max) {
			max = e.EndTime
		}
	}
	return max.Sub(t.StartTime)
}

func unmarshalReport(reportPath string) (*StartupReport, error) {
	// get report.
	reportContent, err := ioutil.ReadFile(reportPath)
	if err != nil {
		return nil, err
	}

	// unmarshal report.
	var report StartupReport
	if err := json.Unmarshal(reportContent, &report); err != nil {
		log.Fatalf("failed to unmarshal report: %s", err)
	}
	return &report, nil
}

// ----------------------------------------------------------------
// Server stuff's
// ----------------------------------------------------------------

//go:embed web
var files embed.FS

// server configs.
var (
	serverPort string
	reportPath string
)

func main() {
	// config.
	loadConfigs()

	// start server.
	log.Fatal(http.ListenAndServe(":"+serverPort, routes()))
}

func loadConfigs() {
	// load configs.
	flag.StringVar(&serverPort, "port", "8080", "server port.")
	flag.StringVar(&reportPath, "report", "", "spring actuator startup report. required!")
	flag.Parse()

	// check report path.
	if reportPath == "" {
		log.Fatal("spring actuator startup report is required!")
	}
}

func routes() *http.ServeMux {
	// create server mux.
	mux := http.NewServeMux()

	// create file server.
	directory, err := fs.Sub(files, "web/static")
	if err != nil {
		log.Fatal(err)
	}
	fileServer := http.FileServer(http.FS(directory))

	// server static files.
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// handle report.
	mux.HandleFunc("/", handleReport)
	return mux
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	// get report.
	report, err := unmarshalReport(reportPath)
	if err != nil {
		log.Printf("failed to unmarshal report: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// set html content type.
	w.Header().Set("Content-Type", "text/html")

	// set funcs.
	funcs := template.FuncMap{
		// classBasedOnDuration returns a css class based on the duration.
		"classBasedOnDuration": func(t time.Duration) string {
			if t > time.Second*5 {
				return "badge-danger"
			}
			if t > time.Second*1 {
				return "badge-warning"
			}
			return "badge-success"
		},
	}

	// load template.
	tpl, err := template.New("").Funcs(funcs).ParseFS(files, "web/index.html")
	if err != nil {
		log.Printf("failed to load template: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// render template.
	err = tpl.ExecuteTemplate(w, "index.html", report)
	if err != nil {
		log.Printf("failed to render template: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
