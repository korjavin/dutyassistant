package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	args := "Fix the door knob /Crusader /3d"

	intervalDays := 0
	specificUserName := ""

	for {
		suffixRe := regexp.MustCompile(`\s+/([^\s/]+)$`)
		match := suffixRe.FindStringSubmatch(args)
		if len(match) > 1 {
			suffix := match[1]

			intervalRe := regexp.MustCompile(`^(?i)([1-9][0-9]*)d$`)
			if intervalMatch := intervalRe.FindStringSubmatch(suffix); len(intervalMatch) > 1 {
				intervalDays, _ = strconv.Atoi(intervalMatch[1])
				args = strings.TrimSpace(args[:len(args)-len(match[0])])
				continue
			}

			specificUserName = suffix
			args = strings.TrimSpace(args[:len(args)-len(match[0])])
			continue
		}
		break
	}

	fmt.Printf("Desc: %q, user: %q, interval: %d\n", args, specificUserName, intervalDays)
}
