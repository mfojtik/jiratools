package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/pterm/pterm"
	flag "github.com/spf13/pflag"
)

func blockerString(values []string) (string, bool) {
	var validValues = []string{
		"blocker?",
		"blocker-",
		"blocker+",
	}
	for _, v := range values {
		for _, validValue := range validValues {
			if v == validValue {
				return v, true
			}
		}
	}
	return "", false
}

func removeBugzillaID(summary string) string {
	i := strings.Index(summary, "]")
	switch {
	case i == -1:
		return summary
	default:
		return strings.TrimSpace(summary[i+1:])
	}
}

func teamNames(config map[string][]string) string {
	result := []string{}
	for name := range config {
		result = append(result, name)
	}
	return strings.Join(result, ",")
}

func teamQuery(names []string) string {
	components := []string{}
	for _, t := range names {
		teamComponents, ok := teamsConfig[t]
		if !ok {
			log.Fatalf("team %q is not configured, only %s are supported", t, teamNames(teamsConfig))
		}
		components = append(components, teamComponents...)
	}
	return fmt.Sprintf(" AND component in (%s)", strings.Join(components, ","))
}

type bug struct {
	summary       string
	component     string
	version       string
	targetVersion string
	blockerFlag   string
	bugzillaLink  string
	priority      string
	status        string
}

var teamsConfig = map[string][]string{
	"api": {
		"kube-apiserver",
		"config-operator",
		"openshift-apiserver",
	},
}

var (
	versions       []string
	targetVersions []string
	blockers       []string
	blockersOnly   bool
	teams          []string
	bugStatus      []string
	bugNotStatus   []string
	counts         bool
)

func init() {
	flag.StringSliceVar(&versions, "versions", []string{}, "comma separated list of version (affected) values (eg. '4.10,4.11'), default is all versions")
	flag.StringSliceVar(&targetVersions, "target-version", []string{}, "comma separated list of target version (fixed in) values (eg. '---,4.11.z'), default is all versions")
	flag.StringSliceVar(&blockers, "blocker", []string{}, "comma separated list of valid blocker bug flag value (eg. '+,-,?'), default is all bugs")
	flag.BoolVar(&blockersOnly, "blockers-only", true, "if set, the list will include bugs without blocker bug flag set")
	flag.StringSliceVar(&teams, "teams", []string{}, fmt.Sprintf("only show bugs for components owned by given team name (valid teams: %q), default is all teams", teamNames(teamsConfig)))
	flag.StringSliceVar(&bugStatus, "status", []string{}, fmt.Sprintf("only include bugs that has this status"))
	flag.StringSliceVar(&bugNotStatus, "not-status", []string{}, fmt.Sprintf("only include bugs that has not this status"))
	flag.BoolVar(&counts, "counts", false, "show counts")
}

func main() {
	flag.Parse()

	tp := jira.PATAuthTransport{Token: os.Getenv("JIRA_TOKEN")}
	client, err := jira.NewClient(tp.Client(), "https://issues.redhat.com/")
	if err != nil {
		log.Fatalf("error getting jira client: %v", err)
	}
	query := "project = OCPBUGSM AND issuetype = Bug" + teamQuery(teams)
	result, _, err := client.Issue.Search(query, &jira.SearchOptions{
		StartAt:    0,
		MaxResults: 200,
	})
	if err != nil {
		log.Fatalf("jira hates you: %v (query: %s)", err, query)
	}

	bugs := make([]bug, len(result))

	for i, issue := range result {
		components := []string{}
		for _, c := range issue.Fields.Components {
			components = append(components, c.Name)
		}
		versions := []string{}
		for _, v := range issue.Fields.AffectsVersions {
			versions = append(versions, v.Name)
		}
		targetVersions := []string{}
		for _, v := range issue.Fields.FixVersions {
			targetVersions = append(targetVersions, v.Name)
		}

		blocker := ""
		flags, err := issue.Fields.Unknowns.StringSlice("customfield_12318640")
		if err == nil {
			if b, hasBlocker := blockerString(flags); hasBlocker {
				blocker = b
			}
		}
		bugzillaLink, err := issue.Fields.Unknowns.String("customfield_12317325")
		if err != nil {
			bugzillaLink = "<unknown>"
		}

		bugs[i] = bug{
			summary:       removeBugzillaID(issue.Fields.Summary),
			component:     strings.Join(components, ","),
			version:       strings.Join(versions, ","),
			targetVersion: strings.Join(targetVersions, ","),
			priority:      getPriorityString(issue.Fields.Priority),
			blockerFlag:   blocker,
			bugzillaLink:  bugzillaLink,
			status:        issue.Fields.Status.Name,
		}
	}

	var list []pterm.BulletListItem

	if hasBlocker("+") {
		list = append(list, pterm.BulletListItem{Level: 0, Text: "blocker+ bugs", TextStyle: pterm.NewStyle(pterm.FgRed)})
		list = append(list, bugsToBulletListItem(bugsWithVersion(targetVersions, versions, bugsWithBlocker("blocker+", bugs)), 1)...)
	}
	if hasBlocker("-") {
		list = append(list, pterm.BulletListItem{Level: 0, Text: "blocker- bugs", TextStyle: pterm.NewStyle(pterm.FgGreen)})
		list = append(list, bugsToBulletListItem(bugsWithVersion(targetVersions, versions, bugsWithBlocker("blocker-", bugs)), 1)...)
	}
	if hasBlocker("?") {
		list = append(list, pterm.BulletListItem{Level: 0, Text: "blocker? bugs", TextStyle: pterm.NewStyle(pterm.FgGray)})
		list = append(list, bugsToBulletListItem(bugsWithVersion(targetVersions, versions, bugsWithBlocker("blocker?", bugs)), 1)...)
	}
	if hasBlocker("") && !blockersOnly {
		list = append(list, pterm.BulletListItem{Level: 0, Text: "no blocker bugs", TextStyle: pterm.NewStyle(pterm.FgGray)})
		list = append(list, bugsToBulletListItem(bugsWithVersion(targetVersions, versions, bugsWithBlocker("", bugs)), 1)...)
	}

	pterm.DefaultBulletList.WithItems(list).Render()

	if counts {
		printBugCounts(bugsWithVersion(targetVersions, versions, bugs))
	}
}

func printBugCounts(bugs []bug) {
	counts := map[string]int64{}
	for _, b := range bugs {
		counts[b.status]++
	}
	result := []string{}
	for status, count := range counts {
		result = append(result, fmt.Sprintf(" > %s = %d", status, count))
	}
	fmt.Printf("%s\n-> Total: %d\n", strings.Join(result, "\n"), len(bugs))
}

func getPriorityString(priority *jira.Priority) string {
	if priority == nil {
		return ""
	}
	v := strings.ToLower(priority.Name)
	// TODO: too many characters jira, too many characters!
	if v == "unprioritized" {
		return ""
	}
	return strings.ToLower(priority.Name)
}

func hasStatus(statuses []string, status string) bool {
	if len(statuses) == 0 {
		return true
	}
	for _, s := range statuses {
		if s == status {
			return true
		}
	}
	return false
}

func hasBlocker(flag string) bool {
	if len(blockers) == 0 {
		return true
	}
	for _, b := range blockers {
		if b == flag {
			return true
		}
	}
	return false
}

func hasVersion(version string, values []string) bool {
	for i := range values {
		if values[i] == version {
			return true
		}
	}
	return false
}

func bugsWithVersion(targetVersion, version []string, bugs []bug) []bug {
	if len(targetVersion) == 0 && len(version) == 0 {
		return bugs
	}
	result := []bug{}
	for i := range bugs {
		if len(targetVersion) > 0 {
			if hasVersion(bugs[i].targetVersion, targetVersions) {
				result = append(result, bugs[i])
			}
		}
		if len(version) > 0 {
			if hasVersion(bugs[i].version, versions) {
				result = append(result, bugs[i])
			}
		}
	}
	return result
}

func bugsWithBlocker(flagValue string, bugs []bug) []bug {
	result := []bug{}
	for i := range bugs {
		if bugs[i].blockerFlag == flagValue {
			result = append(result, bugs[i])
		}
	}
	return result
}

func colorByPriority(priority string) *pterm.Style {
	switch priority {
	case "urgent":
		return pterm.NewStyle(pterm.FgLightRed, pterm.Bold)
	case "high":
		return pterm.NewStyle(pterm.FgRed)
	default:
		return nil
	}
}

func parseStatus(status string) string {
	return strings.ToLower(strings.ReplaceAll(status, " ", ""))
}

func bugsToBulletListItem(bugs []bug, level int) []pterm.BulletListItem {
	result := make([]pterm.BulletListItem, len(bugs))
	for i := range bugs {
		result[i] = pterm.BulletListItem{
			Level:       level,
			Text:        fmt.Sprintf("%s | %s | [%s][%s][%s]", bugs[i].bugzillaLink, bugs[i].summary, bugs[i].targetVersion, bugs[i].priority, parseStatus(bugs[i].status)),
			TextStyle:   colorByPriority(bugs[i].priority),
			Bullet:      "",
			BulletStyle: nil,
		}
	}
	return result
}
