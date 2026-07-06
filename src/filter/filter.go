package filter

import (
    "os"
    "regexp"
    "bufio"
	"log/slog"
	"fmt"
    "strings"
)

type Filter struct {
    WhiteRegex     *regexp.Regexp
    BlackRegex     *regexp.Regexp
    WhitePatterns  []string
    BlackPatterns  []string
}

func New(whitePath, blackPath string) (filter *Filter, err error) {
    wlFile, err := os.Open(whitePath)
    if err != nil {
		slog.Debug(fmt.Sprintf("Couldn't open the whitelist file at path: %s", whitePath))
        return
    } else {
		slog.Debug(fmt.Sprintf("Successfully opened the whitelist file at path: %s", whitePath))
	}
    defer wlFile.Close()

    blFile, err := os.Open(blackPath)
    if err != nil {
		slog.Debug(fmt.Sprintf("Couldn't open the blacklist file at path: %s", blackPath))
        return
    } else {
		slog.Debug(fmt.Sprintf("Successfully opened the blacklist file at path: %s", blackPath))
	}
    defer blFile.Close()

    wlScanner := bufio.NewScanner(wlFile)
    blScanner := bufio.NewScanner(blFile)

	filter = &Filter {
		WhitePatterns: make([]string, 0),
    	BlackPatterns: make([]string, 0),
	}

    var line string
    for wlScanner.Scan() {
        line = wlScanner.Text()
        _, err = regexp.Compile(line)
        if err != nil {
            slog.Warn(fmt.Sprintf("Whitelist pattern is invalid, therefore removed: %s\n", line))
            continue
        }
        filter.WhitePatterns = append(filter.WhitePatterns, line)
    }

    for blScanner.Scan() {
        line = blScanner.Text()
        _, err = regexp.Compile(line)
        if err != nil {
            slog.Warn(fmt.Sprintf("Blacklist pattern is invalid, therefore removed: %s\n", line))
            continue
        }
        filter.BlackPatterns = append(filter.BlackPatterns, line)
    }

	err = nil

    if len(filter.WhitePatterns) > 0 {
        filter.WhiteRegex = regexp.MustCompile("^" + strings.Join(filter.WhitePatterns, "$|^") + "$")
    } else {
        filter.WhiteRegex = nil
    }

	if len(filter.BlackPatterns) > 0 {
		fmt.Printf("%s\n", "^" + strings.Join(filter.BlackPatterns, "$|^") + "$")
    	filter.BlackRegex = regexp.MustCompile("^" + strings.Join(filter.BlackPatterns, "|") + "$")
	} else {
    	filter.BlackRegex = nil
	}

    return
}

func (filter *Filter) Match(text string) bool {
    return  (filter.BlackRegex == nil || !filter.BlackRegex.MatchString(text)) &&
    		filter.WhiteRegex != nil &&
    		filter.WhiteRegex.MatchString(text)
}
