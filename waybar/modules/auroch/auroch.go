package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/mikkeloscar/aur"
	"github.com/nboughton/dotfiles/waybar/modules/gobar"
)

const (
	npm         = "npm"
	github      = "github"
	errClass    = "error"
	npmRegistry = "https://registry.npmjs.org/"
)

var outfile = fmt.Sprintf("%s/tmp/auroch.json", os.Getenv("HOME"))

// Define relevant package data
type pkg struct {
	AurName      string `json:"aur_name,omitempty"` // Name on the AUR (i.e vue-cli)
	AurVer       string `json:"aur_ver,omitempty"`
	UpstreamName string `json:"upstream_name,omitempty"` // Name on the upstream source (i.e @vue/cli)
	UpstreamType string `json:"upstream_type,omitempty"` // Upstream type: npm|github
	UpstreamVer  string `json:"upstream_ver,omitempty"`
}

// Just the bit of NPM registry info that I care about
type npmInfo struct {
	Versions map[string]interface{} `json:"versions,omitempty"`
}

// Retrieve the version of the current AUR package
func (p *pkg) getAurVer() error {
	// Request pkg info
	res, err := aur.Info([]string{p.AurName})
	if err != nil {
		return err
	}

	// Copy it to pkg struct
	p.AurVer = res[0].Version

	// Strip revision
	p.AurVer = regexp.MustCompile(`-\d$`).ReplaceAllString(p.AurVer, "")

	return nil
}

func (p *pkg) getNpmVer() error {
	// Request and decode the versions list
	n := npmInfo{}
	resp, err := http.Get(fmt.Sprintf("%s/%s", npmRegistry, p.UpstreamName))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return err
	}

	// Copy the keys into an array and sort it, last item should be most recent
	v := []string{}
	for key := range n.Versions {
		v = append(v, key)
	}

	sort.Strings(v)
	p.UpstreamVer = v[len(v)-1]

	return nil
}

func main() {
	packages := []*pkg{
		{
			AurName:      "vue-cli",
			UpstreamName: "@vue/cli",
			UpstreamType: npm,
		},
		{
			AurName:      "vue-cli-service-global",
			UpstreamName: "@vue/cli-service-global",
			UpstreamType: npm,
		},
		{
			AurName:      "quasar-cli",
			UpstreamName: "@quasar/cli",
			UpstreamType: npm,
		},
		{
			AurName:      "quasar-icongenie",
			UpstreamName: "@quasar/icongenie",
			UpstreamType: npm,
		},
	}

	var (
		out   []string
		class string
		err   error
	)

	class = "no-updates"
	for _, p := range packages {
		log.Printf("Checking package: %s\n", p.AurName)
		if err = p.getAurVer(); err != nil {
			class = errClass
			break

		}

		// I'll add a switch or if block here to retrieve ver data from non-npm sources when I have some to care about
		if err = p.getNpmVer(); err != nil {
			class = errClass
			break
		}

		log.Printf("%s %s -> %s\n", p.AurName, p.AurVer, p.UpstreamVer)
		if p.AurVer != p.UpstreamVer {
			out = append(out, fmt.Sprintf("%s %s -> %s", p.AurName, p.AurVer, p.UpstreamVer))
		}
	}

	n := len(out)

	txt := fmt.Sprintf("%d", n)
	if class == errClass {
		txt = "!"
	}

	if n > 0 {
		class = "updates"
	}

	m := gobar.Module{
		Name:    "AUROCH",
		Summary: "Outdated AUR Packages",
		JSON: gobar.JSONOutput{
			Text:       txt,
			Alt:        txt,
			Class:      class,
			Tooltip:    strings.Join(out, "\n"),
			Percentage: n,
		},
	}

	if n > 0 {
		log.Println("Sending DBUS Notification")
		m.Notify(m.JSON.Tooltip, 10000)
	}

	log.Println("Writing JSON data")
	f, err := os.Create(outfile)
	if err != nil {
		log.Fatalf("could not open %s for writing", outfile)
	}
	defer f.Close()
	m.JSON.Write(f)
}