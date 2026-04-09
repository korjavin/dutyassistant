package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	args := "Fix the door knob /Crusader /3d"

	// mock list of users
	userNames := map[string]bool{"crusader": true, "alice": true, "bob": true}

	var intervalDays int
	var specificUserName string
	description := args

	for {
		suffixRe := regexp.MustCompile(`(?i)\s+/([^\s/]+)$`)
		match := suffixRe.FindStringSubmatch(description)
		if len(match) > 1 {
			suffix := match[1]

			intervalRe := regexp.MustCompile(`^(?i)([1-9][0-9]*)d$`)
			if intervalMatch := intervalRe.FindStringSubmatch(suffix); len(intervalMatch) > 1 {
				intervalDays, _ = strconv.Atoi(intervalMatch[1])
				description = strings.TrimSpace(description[:len(description)-len(match[0])])
				continue
			}

			if userNames[strings.ToLower(suffix)] {
				specificUserName = suffix
				description = strings.TrimSpace(description[:len(description)-len(match[0])])
				continue
			}
		}
		break
	}

	fmt.Printf("Desc: %q, user: %q, interval: %d\n", description, specificUserName, intervalDays)
}
